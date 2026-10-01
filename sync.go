package main

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

type App struct {
	Git         Git
	Backlog     Backlog
	GitHub      GitHub
	Logf        func(string, ...any)
	verboseSeen map[string]bool
}

func (a *App) Run(ctx context.Context, cfg Config) (Counters, error) {
	if a.Logf == nil {
		a.Logf = func(string, ...any) {}
	}
	cfg = cfg.Normalized()
	prefix, err := a.Backlog.TaskPrefix(ctx, cfg.Root)
	if err != nil {
		return Counters{}, fmt.Errorf("read task prefix through backlog config: %w", err)
	}
	if strings.TrimSpace(prefix) == "" {
		return Counters{}, fmt.Errorf("read task prefix through backlog config: empty prefix")
	}
	cfg.TaskPrefix = strings.ToLower(strings.TrimSpace(prefix))
	resolved, err := CollectTasks(ctx, cfg, a.Git, a.Backlog, a.Logf)
	if err != nil {
		return Counters{}, err
	}
	milestones, err := a.Backlog.Milestones(ctx, cfg.Root)
	if err != nil {
		a.Logf("warning: milestone list unavailable: %v", err)
		milestones = map[string]string{}
	}
	project, err := a.validateProjectStatusOptions(ctx, cfg, resolved)
	if err != nil {
		return Counters{}, err
	}
	allIssues, err := a.listConfiguredIssues(ctx, cfg)
	if err != nil {
		return Counters{}, err
	}
	markerIssues, _ := IndexIssues(allIssues, cfg.Inbox.Label, cfg.TaskPrefix, a.Logf)
	issueByTaskID := map[string]Issue{}
	claimedIssues := map[string]string{}
	for id, issue := range markerIssues {
		issueByTaskID[id] = issue
		if key := issueClaimKey(issue); key != "" {
			claimedIssues[key] = id
		}
	}
	for _, issue := range allIssues {
		if id, ok := ParseMarkerWithPrefix(issue.Body, cfg.TaskPrefix); ok {
			if key := issueClaimKey(issue); key != "" {
				claimedIssues[key] = id
			}
		}
	}
	var counters Counters
	recordTaskFailure := func(taskID, operation string, err error) {
		counters.Failed++
		a.Logf("error: %s %s failed: %v", CanonicalTaskID(taskID), operation, err)
	}
	if cfg.Inbox.Enabled && !cfg.NoInbox {
		imported, linked, err := a.processInbox(ctx, cfg, allIssues, markerIssues, issueByTaskID, claimedIssues, resolved)
		if err != nil {
			return counters, err
		}
		counters.Imported += imported
		for id, issue := range linked {
			markerIssues[id] = issue
			issueByTaskID[id] = issue
			if key := issueClaimKey(issue); key != "" {
				claimedIssues[key] = id
			}
		}
	}
	a.adoptReferencedIssues(cfg, resolved, allIssues, markerIssues, issueByTaskID, claimedIssues)
	// First pass creates/adopts issues and gets project items. Bodies are intentionally rendered in pass two
	// after every task has an issue mapping, making the next real run a no-op.
	projectItems, err := a.GitHub.ListProjectItems(ctx, project.ID)
	if err != nil {
		return counters, err
	}
	projectByContent := indexProjectItems(projectItems)
	failedCreate := map[string]bool{}
	for _, task := range resolved.Tasks {
		id := CanonicalTaskID(task.ID)
		targetRepo := cfg.TargetRepo(task)
		issue, exists := markerIssues[id]
		if exists && !strings.EqualFold(issue.Repo, targetRepo) {
			a.Logf("warning: %s maps to %s but existing issue is in %s; updating existing issue in place", id, targetRepo, issue.Repo)
		}
		repo := targetRepo
		if exists {
			repo = issue.Repo
		}
		if !exists {
			body := RenderIssueBodyWithOptions(task, RenderOptions{IssueByTaskID: issueByTaskID, TaskByID: resolved.ByID, Milestones: milestones, MainBranch: cfg.MainBranch})
			if err := validateIssueBodyWithinGitHubLimit(id, body); err != nil {
				recordTaskFailure(id, "render issue body", err)
				failedCreate[id] = true
				continue
			}
			labels := DesiredLabels(task, Issue{}, cfg)
			labelFailed := false
			for _, label := range labels {
				if err := a.ensureLabel(ctx, cfg, repo, label); err != nil {
					recordTaskFailure(id, fmt.Sprintf("ensure label %q", label), err)
					labelFailed = true
					break
				}
			}
			if labelFailed {
				failedCreate[id] = true
				continue
			}
			a.LogAction(cfg.DryRun, "create issue %s in %s", IssueTitle(task), repo)
			if !cfg.DryRun {
				created, err := a.GitHub.CreateIssue(ctx, repo, IssueTitle(task), body, labels)
				if err != nil {
					recordTaskFailure(id, "create issue", err)
					failedCreate[id] = true
					continue
				}
				issue = created
				issue.Repo = repo
				markerIssues[id] = created
				issueByTaskID[id] = created
			} else {
				issue = Issue{Repo: repo, Title: IssueTitle(task), Body: body, State: "open", Labels: labelsToIssueLabels(labels)}
				markerIssues[id] = issue
				issueByTaskID[id] = issue
			}
			counters.Created++
		}
	}
	// Refresh project state for real creations.
	if !cfg.DryRun {
		projectItems, err = a.GitHub.ListProjectItems(ctx, project.ID)
		if err != nil {
			return counters, err
		}
		projectByContent = indexProjectItems(projectItems)
	}
	for _, task := range resolved.Tasks {
		id := CanonicalTaskID(task.ID)
		if failedCreate[id] {
			continue
		}
		issue, issueOK := markerIssues[id]
		if !issueOK {
			recordTaskFailure(id, "find issue mapping", fmt.Errorf("no issue found after create/adopt pass"))
			continue
		}
		repo := issue.Repo
		if repo == "" {
			repo = cfg.TargetRepo(task)
		}
		if cfg.DryRun && issue.NodeID == "" && issue.Number == 0 {
			a.Logf("would add new issue for %s to project", id)
			a.Logf("would set project status for new issue %s <unset>→%s", id, projectStatusName(cfg, task.Status))
			counters.ProjectAdded++
			counters.StatusChanges++
			counters.FieldChanges += a.logDryRunFieldSets(cfg, project, task, milestones)
			continue
		}
		body := RenderIssueBodyWithOptions(task, RenderOptions{IssueByTaskID: issueByTaskID, TaskByID: resolved.ByID, Milestones: milestones, MainBranch: cfg.MainBranch})
		if err := validateIssueBodyWithinGitHubLimit(id, body); err != nil {
			recordTaskFailure(id, "render issue body", err)
			continue
		}
		labels := DesiredLabels(task, issue, cfg)
		patch, fields := DiffIssue(task, issue, body, labels)
		if len(fields) > 0 {
			labelFailed := false
			for _, label := range labels {
				if err := a.ensureLabel(ctx, cfg, repo, label); err != nil {
					recordTaskFailure(id, fmt.Sprintf("ensure label %q", label), err)
					labelFailed = true
					break
				}
			}
			if labelFailed {
				continue
			}
			a.LogAction(cfg.DryRun, "update issue %s#%d fields: %s", repo, issue.Number, strings.Join(fields, ", "))
			if !cfg.DryRun {
				updated, err := a.GitHub.UpdateIssue(ctx, repo, issue.Number, patch)
				if err != nil {
					recordTaskFailure(id, "update issue", err)
					continue
				}
				updated.Repo = repo
				issue = updated
				markerIssues[id] = updated
				issueByTaskID[id] = updated
			}
			counters.Updated++
		}
		item, itemOK := projectByContent[issue.NodeID]
		if issue.NodeID == "" {
			if cfg.DryRun {
				a.Logf("would add new issue for %s to project", id)
				a.Logf("would set project status for new issue %s <unset>→%s", id, projectStatusName(cfg, task.Status))
				counters.ProjectAdded++
				counters.StatusChanges++
				counters.FieldChanges += a.logDryRunFieldSets(cfg, project, task, milestones)
			}
			continue
		}
		if !itemOK {
			a.LogAction(cfg.DryRun, "add issue %s#%d to project", repo, issue.Number)
			if !cfg.DryRun {
				added, err := a.GitHub.AddProjectItem(ctx, project.ID, issue.NodeID)
				if err != nil {
					recordTaskFailure(id, "add issue to project", err)
					continue
				}
				item = added
				item.ContentNodeID = issue.NodeID
				projectByContent[issue.NodeID] = item
			} else {
				item = ProjectItem{ID: "DRY-RUN-" + issue.NodeID, ContentNodeID: issue.NodeID, FieldValues: map[string]ProjectFieldValue{}}
			}
			counters.ProjectAdded++
		}
		changed, err := a.syncProjectFields(ctx, cfg, project, item, task, milestones, issue)
		counters.StatusChanges += changed.status
		counters.FieldChanges += changed.fields
		if err != nil {
			recordTaskFailure(id, "sync project fields", err)
			continue
		}
	}
	if cfg.SubIssues {
		linked, failed := a.syncSubIssues(ctx, cfg, resolved, issueByTaskID)
		counters.SubIssueLinks += linked
		counters.Failed += failed
	}
	a.Logf("sync complete: %d created, %d updated, %d status changes, %d imported, %d failed operations", counters.Created, counters.Updated, counters.StatusChanges, counters.Imported, counters.Failed)
	if counters.Failed > 0 {
		return counters, fmt.Errorf("%d task sync operation(s) failed", counters.Failed)
	}
	return counters, nil
}

func (a *App) listConfiguredIssues(ctx context.Context, cfg Config) ([]Issue, error) {
	var all []Issue
	for _, repo := range cfg.ConfiguredRepos() {
		issues, err := a.GitHub.ListIssues(ctx, repo)
		if err != nil {
			return nil, err
		}
		for i := range issues {
			issues[i].Repo = repo
		}
		all = append(all, issues...)
	}
	return all, nil
}

func (a *App) validateProjectStatusOptions(ctx context.Context, cfg Config, resolved ResolvedTasks) (ProjectInfo, error) {
	statuses, err := a.Backlog.Statuses(ctx, cfg.Root)
	if err != nil {
		return ProjectInfo{}, err
	}
	project, err := a.GitHub.ProjectInfo(ctx, cfg.ProjectOwnerType, cfg.ProjectOwner, cfg.ProjectNumber)
	if err != nil {
		return ProjectInfo{}, err
	}
	var missing []string
	for _, status := range statuses {
		name := projectStatusName(cfg, status)
		if optionIDCaseInsensitive(project.StatusOptionID, name) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return ProjectInfo{}, fmt.Errorf("project Status options missing: %v", missing)
	}
	_ = resolved
	return project, nil
}

func projectStatusName(cfg Config, backlogStatus string) string {
	if mapped := cfg.StatusMap[backlogStatus]; mapped != "" {
		return mapped
	}
	for k, v := range cfg.StatusMap {
		if strings.EqualFold(k, backlogStatus) {
			return v
		}
	}
	return backlogStatus
}

func IndexIssues(issues []Issue, inboxLabel, taskPrefix string, logf func(string, ...any)) (map[string]Issue, []Issue) {
	byMarker := map[string]Issue{}
	var inbox []Issue
	for _, issue := range issues {
		if hasLabelFold(issue, inboxLabel) && issue.State == "open" {
			inbox = append(inbox, issue)
		}
		id, ok := ParseMarkerWithPrefix(issue.Body, taskPrefix)
		if !ok {
			continue
		}
		existing, dup := byMarker[id]
		if !dup || issue.Number < existing.Number {
			if dup && logf != nil {
				logf("warning: duplicate marker %s on %s#%d and %s#%d; using lower issue number", id, existing.Repo, existing.Number, issue.Repo, issue.Number)
			}
			byMarker[id] = issue
		} else if logf != nil {
			logf("warning: duplicate marker %s on %s#%d and %s#%d; using lower issue number", id, existing.Repo, existing.Number, issue.Repo, issue.Number)
		}
	}
	return byMarker, inbox
}

func DesiredLabels(task Task, issue Issue, cfg Config) []string {
	inboxLabel := cfg.Inbox.Label
	desired := []string{}
	for _, label := range task.Labels {
		if strings.EqualFold(label, inboxLabel) {
			continue
		}
		if labelAllowedByManaged(label, cfg.Labels.Managed) {
			desired = append(desired, label)
		}
	}
	for _, label := range cfg.Labels.AddAlways {
		if strings.EqualFold(label, inboxLabel) {
			continue
		}
		if labelAllowedByManaged(label, cfg.Labels.Managed) {
			desired = append(desired, label)
		}
	}
	if cfg.Labels.PriorityPrefix != "" && task.Priority != nil && strings.TrimSpace(*task.Priority) != "" {
		label := cfg.Labels.PriorityPrefix + strings.ToLower(strings.TrimSpace(*task.Priority))
		if !strings.EqualFold(label, inboxLabel) && labelAllowedByManaged(label, cfg.Labels.Managed) {
			desired = append(desired, label)
		}
	}
	desired = DedupSorted(desired)
	out := append([]string(nil), desired...)
	desiredSet := lowerSet(desired)
	for _, label := range LabelsOf(issue) {
		if strings.EqualFold(label, inboxLabel) {
			out = append(out, label)
			continue
		}
		if desiredSet[strings.ToLower(label)] {
			continue
		}
		if labelManagedForRemoval(label, cfg.Labels.Managed) {
			continue
		}
		out = append(out, label)
	}
	return DedupSorted(out)
}

func labelAllowedByManaged(label string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}
	for _, p := range patterns {
		if p == "*" {
			return true
		}
		if strings.HasSuffix(p, "*") && strings.HasPrefix(strings.ToLower(label), strings.ToLower(strings.TrimSuffix(p, "*"))) {
			return true
		}
		if strings.EqualFold(label, p) {
			return true
		}
	}
	return false
}
func labelManagedForRemoval(label string, patterns []string) bool {
	for _, p := range patterns {
		if p == "*" {
			continue
		}
		if strings.HasSuffix(p, "*") && strings.HasPrefix(strings.ToLower(label), strings.ToLower(strings.TrimSuffix(p, "*"))) {
			return true
		}
		if strings.EqualFold(label, p) {
			return true
		}
	}
	return false
}
func lowerSet(values []string) map[string]bool {
	m := map[string]bool{}
	for _, v := range values {
		m[strings.ToLower(v)] = true
	}
	return m
}

func DiffIssue(task Task, issue Issue, body string, labels []string) (IssuePatch, []string) {
	var patch IssuePatch
	var fields []string
	title := IssueTitle(task)
	if issue.Title != title {
		patch.Title = &title
		fields = append(fields, "title")
	}
	if issue.Body != body {
		patch.Body = &body
		fields = append(fields, "body")
	}
	if !EqualStringSlicesFold(LabelsOf(issue), labels, true) {
		patch.Labels = &labels
		fields = append(fields, "labels")
	}
	desiredState := "open"
	if task.Status == "Done" {
		desiredState = "closed"
	}
	if issue.State != desiredState {
		patch.State = &desiredState
		fields = append(fields, "state")
		if desiredState == "closed" {
			reason := "completed"
			patch.StateReason = &reason
		}
	}
	return patch, fields
}

func (a *App) processInbox(ctx context.Context, cfg Config, issues []Issue, markerIssues map[string]Issue, issueByTaskID map[string]Issue, claimedIssues map[string]string, resolved ResolvedTasks) (imported int, linked map[string]Issue, err error) {
	clean, reason, err := a.Git.RootBranchClean(ctx, cfg.Root, cfg.MainBranch)
	if err != nil {
		return 0, nil, err
	}
	linked = map[string]Issue{}
	defer func() {
		if imported > 0 && cfg.Inbox.Push && !cfg.DryRun {
			if pushErr := a.Backlog.Push(ctx, cfg.Root, cfg.MainBranch); pushErr != nil {
				a.Logf("warning: push %s failed after inbox import: %v", cfg.MainBranch, pushErr)
			}
		}
	}()
	for _, issue := range issues {
		if issue.PullRequest != nil || issue.State != "open" || !hasLabelFold(issue, cfg.Inbox.Label) {
			continue
		}
		if id, ok := ParseMarkerWithPrefix(issue.Body, cfg.TaskPrefix); ok {
			labels := removeLabelFold(LabelsOf(issue), cfg.Inbox.Label)
			patch := IssuePatch{Labels: &labels}
			a.LogAction(cfg.DryRun, "remove inbox label from mirrored issue %s#%d (%s)", issue.Repo, issue.Number, id)
			updated := issue
			if !cfg.DryRun {
				updated, err = a.GitHub.UpdateIssue(ctx, issue.Repo, issue.Number, patch)
				if err != nil {
					return imported, linked, err
				}
			}
			updated.Repo = issue.Repo
			markerIssues[id] = updated
			issueByTaskID[id] = updated
			linked[id] = updated
			if key := issueClaimKey(updated); key != "" {
				claimedIssues[key] = id
			}
			continue
		}
		if !clean {
			a.Logf("skip inbox issue %s#%d: %s", issue.Repo, issue.Number, reason)
			continue
		}
		issueURL := issue.HTMLURL
		if issueURL == "" {
			issueURL = fmt.Sprintf("https://github.com/%s/issues/%d", issue.Repo, issue.Number)
		}
		taskID := findTaskByReference(resolved.Tasks, issueURL)
		canonicalTaskID := CanonicalTaskID(taskID)
		if taskID != "" {
			if existing, ok := markerIssues[canonicalTaskID]; ok && !sameIssue(existing, issue) {
				a.Logf("warning: refusing to mark inbox issue %s#%d for %s because %s already has %s#%d; removing only inbox label", issue.Repo, issue.Number, canonicalTaskID, canonicalTaskID, existing.Repo, existing.Number)
				if err := a.stripInboxLabel(ctx, cfg, issue); err != nil {
					return imported, linked, err
				}
				continue
			}
			if owner, claimed := claimedIssues[issueClaimKey(issue)]; claimed && owner != canonicalTaskID {
				a.Logf("warning: refusing to mark inbox issue %s#%d for %s because it is already claimed by %s; removing only inbox label", issue.Repo, issue.Number, canonicalTaskID, owner)
				if err := a.stripInboxLabel(ctx, cfg, issue); err != nil {
					return imported, linked, err
				}
				continue
			}
		}
		if taskID == "" {
			description := strings.TrimSpace(issue.Body)
			project := cfg.ProjectForRepo(issue.Repo)
			if cfg.DryRun {
				a.Logf("would create task from %s#%d", issue.Repo, issue.Number)
				taskID = strings.ToUpper(cfg.TaskPrefix) + "-DRY-RUN"
			} else {
				id, err := a.Backlog.CreateTask(ctx, cfg.Root, CreateTaskInput{Title: issue.Title, Description: description, Labels: []string{}, Project: project, References: []string{issueURL}, TaskPrefix: cfg.TaskPrefix})
				if err != nil {
					return imported, linked, err
				}
				taskID = id
				imported++
			}
		} else {
			a.Logf("reuse existing referenced task %s for issue %s#%d", CanonicalTaskID(taskID), issue.Repo, issue.Number)
		}
		if cfg.DryRun {
			continue
		}
		body := MarkerFor(taskID) + "\n" + issue.Body
		title := fmt.Sprintf("%s: %s", CanonicalTaskID(taskID), issue.Title)
		labels := removeLabelFold(LabelsOf(issue), cfg.Inbox.Label)
		patch := IssuePatch{Title: &title, Body: &body, Labels: &labels}
		updated, err := a.GitHub.UpdateIssue(ctx, issue.Repo, issue.Number, patch)
		if err != nil {
			return imported, linked, err
		}
		updated.Repo = issue.Repo
		linked[CanonicalTaskID(taskID)] = updated
		markerIssues[CanonicalTaskID(taskID)] = updated
		issueByTaskID[CanonicalTaskID(taskID)] = updated
		if key := issueClaimKey(updated); key != "" {
			claimedIssues[key] = CanonicalTaskID(taskID)
		}
	}
	return imported, linked, nil
}

func (a *App) stripInboxLabel(ctx context.Context, cfg Config, issue Issue) error {
	labels := removeLabelFold(LabelsOf(issue), cfg.Inbox.Label)
	patch := IssuePatch{Labels: &labels}
	a.LogAction(cfg.DryRun, "remove inbox label from extra issue %s#%d", issue.Repo, issue.Number)
	if cfg.DryRun {
		return nil
	}
	_, err := a.GitHub.UpdateIssue(ctx, issue.Repo, issue.Number, patch)
	return err
}

func issueClaimKey(issue Issue) string {
	if issue.Repo == "" || issue.Number == 0 {
		return ""
	}
	return strings.ToLower(fmt.Sprintf("%s#%d", issue.Repo, issue.Number))
}

func sameIssue(a, b Issue) bool {
	return a.Number != 0 && b.Number != 0 && strings.EqualFold(a.Repo, b.Repo) && a.Number == b.Number
}

func findTaskByReference(tasks []Task, issueURL string) string {
	for _, task := range tasks {
		for _, ref := range task.References {
			if strings.EqualFold(ref, issueURL) {
				return task.ID
			}
		}
	}
	return ""
}

func (a *App) adoptReferencedIssues(cfg Config, resolved ResolvedTasks, issues []Issue, markerIssues map[string]Issue, issueByTaskID map[string]Issue, claimedIssues map[string]string) {
	if !cfg.AdoptReferencedIssues {
		return
	}
	byRepoNum := map[string]Issue{}
	for _, issue := range issues {
		byRepoNum[strings.ToLower(fmt.Sprintf("%s#%d", issue.Repo, issue.Number))] = issue
	}
	for _, task := range resolved.Tasks {
		id := CanonicalTaskID(task.ID)
		if _, exists := markerIssues[id]; exists {
			continue
		}
		target := cfg.TargetRepo(task)
		var nums []int
		for _, ref := range task.References {
			repo, num, ok := parseIssueURL(ref)
			if ok && strings.EqualFold(repo, target) {
				nums = append(nums, num)
			}
		}
		if len(nums) == 0 {
			continue
		}
		sort.Ints(nums)
		if len(nums) > 1 {
			a.Logf("warning: multiple adoptable references for %s; considering lowest unclaimed issue", id)
		}
		var chosen Issue
		for _, num := range nums {
			issue, ok := byRepoNum[strings.ToLower(fmt.Sprintf("%s#%d", target, num))]
			if !ok {
				continue
			}
			key := issueClaimKey(issue)
			if owner, claimed := claimedIssues[key]; claimed && owner != id {
				a.Logf("warning: refusing to adopt %s#%d for %s because it is already claimed by %s", target, num, id, owner)
				continue
			}
			chosen = issue
			break
		}
		if chosen.Number == 0 {
			continue
		}
		if other, ok := ParseMarkerWithPrefix(chosen.Body, cfg.TaskPrefix); ok && other != id {
			a.Logf("warning: refusing to adopt %s#%d for %s because it already has marker %s", target, chosen.Number, id, other)
			continue
		}
		a.LogAction(cfg.DryRun, "adopt referenced issue %s#%d for %s", target, chosen.Number, id)
		markerIssues[id] = chosen
		issueByTaskID[id] = chosen
		if key := issueClaimKey(chosen); key != "" {
			claimedIssues[key] = id
		}
	}
}

func parseIssueURL(raw string) (string, int, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Host != "github.com" {
		return "", 0, false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 4 || parts[2] != "issues" {
		return "", 0, false
	}
	n, err := strconv.Atoi(parts[3])
	if err != nil {
		return "", 0, false
	}
	return parts[0] + "/" + parts[1], n, true
}

type projectChangeCounts struct{ status, fields int }

func (a *App) syncProjectFields(ctx context.Context, cfg Config, project ProjectInfo, item ProjectItem, task Task, milestones map[string]string, issue Issue) (projectChangeCounts, error) {
	var counts projectChangeCounts
	if item.ID == "" {
		return counts, nil
	}
	statusName := projectStatusName(cfg, task.Status)
	optionID := optionIDCaseInsensitive(project.StatusOptionID, statusName)
	if optionID != "" && item.StatusOptionID != optionID {
		old := item.Status
		if old == "" {
			old = "<unset>"
		}
		a.LogAction(cfg.DryRun, "set project status for issue %s#%d %s→%s", issue.Repo, issue.Number, old, statusName)
		if !cfg.DryRun {
			if err := a.GitHub.UpdateProjectStatus(ctx, project.ID, item.ID, project.StatusFieldID, optionID); err != nil {
				return counts, err
			}
		}
		counts.status++
	}
	sets := desiredProjectFieldValues(cfg, task, milestones)
	for cfgName, desired := range sets {
		if desired.fieldName == "" {
			continue
		}
		field, ok := project.Fields[desired.fieldName]
		if !ok {
			a.verboseOnce(cfg, "field-missing-"+desired.fieldName, "verbose: project field %q is absent; skipping", desired.fieldName)
			continue
		}
		current := item.FieldValues[desired.fieldName]
		if desired.kind == "text" {
			if desired.text == "" {
				if current.Text == "" {
					continue
				}
				a.LogAction(cfg.DryRun, "clear project field %q for issue %s#%d", desired.fieldName, issue.Repo, issue.Number)
				if !cfg.DryRun {
					if err := a.GitHub.ClearProjectField(ctx, project.ID, item.ID, field.ID); err != nil {
						return counts, err
					}
				}
			} else if current.Text != desired.text {
				a.LogAction(cfg.DryRun, "set project field %q for issue %s#%d to %q", desired.fieldName, issue.Repo, issue.Number, desired.text)
				if !cfg.DryRun {
					if err := a.GitHub.UpdateProjectText(ctx, project.ID, item.ID, field.ID, desired.text); err != nil {
						return counts, err
					}
				}
			} else {
				continue
			}
			counts.fields++
		} else {
			if desired.text == "" {
				if current.OptionID == "" && current.Name == "" {
					continue
				}
				a.LogAction(cfg.DryRun, "clear project field %q for issue %s#%d", desired.fieldName, issue.Repo, issue.Number)
				if !cfg.DryRun {
					if err := a.GitHub.ClearProjectField(ctx, project.ID, item.ID, field.ID); err != nil {
						return counts, err
					}
				}
				counts.fields++
				continue
			}
			optionID := optionIDCaseInsensitive(field.Options, desired.text)
			if optionID == "" {
				a.Logf("warning: project field %q has no option %q; not auto-adding options to avoid clearing existing values", desired.fieldName, desired.text)
				continue
			}
			if current.OptionID == optionID {
				continue
			}
			a.LogAction(cfg.DryRun, "set project field %q for issue %s#%d to %q", desired.fieldName, issue.Repo, issue.Number, desired.text)
			if !cfg.DryRun {
				if err := a.GitHub.UpdateProjectSingleSelect(ctx, project.ID, item.ID, field.ID, optionID); err != nil {
					return counts, err
				}
			}
			counts.fields++
		}
		_ = cfgName
	}
	return counts, nil
}

type desiredField struct{ fieldName, kind, text string }

func desiredProjectFieldValues(cfg Config, task Task, milestones map[string]string) map[string]desiredField {
	branch := task.Branch
	if branch == cfg.MainBranch {
		branch = ""
	}
	values := map[string]desiredField{
		"priority":  {cfg.Fields.Priority, "single", stringPtrOrNone(task.Priority)},
		"milestone": {cfg.Fields.Milestone, "single", milestoneTitle(task.Milestone, milestones)},
		"area":      {cfg.Fields.Area, "single", stringPtrOrNone(task.Project)},
		"taskId":    {cfg.Fields.TaskID, "text", CanonicalTaskID(task.ID)},
		"branch":    {cfg.Fields.Branch, "text", branch},
	}
	for key, value := range values {
		if value.text == "None" {
			value.text = ""
			values[key] = value
		}
	}
	return values
}
func optionIDCaseInsensitive(options map[string]string, name string) string {
	for opt, id := range options {
		if strings.EqualFold(opt, name) {
			return id
		}
	}
	return ""
}

func validateIssueBodyWithinGitHubLimit(taskID, body string) error {
	chars := countCharacters(body)
	if chars <= githubIssueBodyCharacterLimit {
		return nil
	}
	return fmt.Errorf("rendered issue body is %d characters after truncation; GitHub hard limit is %d", chars, githubIssueBodyCharacterLimit)
}

func (a *App) logDryRunFieldSets(cfg Config, project ProjectInfo, task Task, milestones map[string]string) int {
	count := 0
	for _, desired := range desiredProjectFieldValues(cfg, task, milestones) {
		if desired.fieldName == "" || desired.text == "" {
			continue
		}
		if _, ok := project.Fields[desired.fieldName]; !ok {
			continue
		}
		a.Logf("would set project field %q for new issue %s to %q", desired.fieldName, CanonicalTaskID(task.ID), desired.text)
		count++
	}
	return count
}

func (a *App) syncSubIssues(ctx context.Context, cfg Config, resolved ResolvedTasks, issueByTaskID map[string]Issue) (linked int, failed int) {
	warned := map[string]bool{}
	for _, task := range resolved.Tasks {
		id := CanonicalTaskID(task.ID)
		if task.ParentTaskID == nil || strings.TrimSpace(*task.ParentTaskID) == "" {
			continue
		}
		child, ok := issueByTaskID[id]
		if !ok || child.NodeID == "" || child.DatabaseID == 0 {
			continue
		}
		parent, ok := issueByTaskID[CanonicalTaskID(*task.ParentTaskID)]
		if !ok || parent.NodeID == "" {
			continue
		}
		current, err := a.GitHub.IssueParent(ctx, child.NodeID)
		if err != nil {
			failed++
			a.Logf("error: %s read sub-issue parent failed: %v", id, err)
			continue
		}
		if current.ID == parent.NodeID {
			continue
		}
		if current.ID != "" {
			if !cfg.RepoAllowed(current.Repo) {
				a.warnOnce(warned, "foreign-parent-"+current.Repo, "warning: refusing to remove foreign sub-issue parent %s#%d for %s#%d; skipping link", current.Repo, current.Number, child.Repo, child.Number)
				continue
			}
			a.LogAction(cfg.DryRun, "remove existing sub-issue parent %s#%d for %s#%d", current.Repo, current.Number, child.Repo, child.Number)
		}
		a.LogAction(cfg.DryRun, "link %s#%d under parent %s#%d", child.Repo, child.Number, parent.Repo, parent.Number)
		if !cfg.DryRun {
			if current.ID != "" {
				if err := a.GitHub.RemoveSubIssue(ctx, current.ID, child.NodeID); err != nil {
					failed++
					a.Logf("error: %s remove sub-issue parent %s#%d failed: %v", id, current.Repo, current.Number, err)
					continue
				}
			}
			if err := a.GitHub.AddSubIssue(ctx, parent.Repo, parent.Number, child.DatabaseID); err != nil {
				if isExpectedSubIssueLinkRejection(err) {
					a.warnOnce(warned, "add-sub-issue-"+strings.ToLower(err.Error()), "warning: sub-issue links are not supported for %s#%d under %s#%d; skipping link: %v", child.Repo, child.Number, parent.Repo, parent.Number, err)
					continue
				}
				failed++
				a.Logf("error: %s add sub-issue under %s#%d failed: %v", id, parent.Repo, parent.Number, err)
				continue
			}
		}
		linked++
	}
	return linked, failed
}

func isExpectedSubIssueLinkRejection(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, part := range []string{"unsupported", "not supported", "not support", "cross-repo", "cross repo", "different repositories", "same repository"} {
		if strings.Contains(msg, part) {
			return true
		}
	}
	return false
}

func (a *App) ensureLabel(ctx context.Context, cfg Config, repo, label string) error {
	if cfg.DryRun {
		a.Logf("would create label %q in %s if missing", label, repo)
		return nil
	}
	return a.GitHub.EnsureLabel(ctx, repo, label)
}
func (a *App) LogAction(dryRun bool, format string, args ...any) {
	prefix := ""
	if dryRun {
		prefix = "would "
	}
	msg := fmt.Sprintf(format, args...)
	if dryRun && strings.HasPrefix(msg, "would ") {
		a.Logf("%s", msg)
		return
	}
	a.Logf("%s%s", prefix, msg)
}
func (a *App) verboseOnce(cfg Config, key, format string, args ...any) {
	if !cfg.Verbose {
		return
	}
	if a.verboseSeen == nil {
		a.verboseSeen = map[string]bool{}
	}
	if a.verboseSeen[key] {
		return
	}
	a.verboseSeen[key] = true
	a.Logf(format, args...)
}
func (a *App) warnOnce(seen map[string]bool, key, format string, args ...any) {
	if seen[key] {
		return
	}
	seen[key] = true
	a.Logf(format, args...)
}

func hasLabelFold(issue Issue, label string) bool {
	for _, item := range issue.Labels {
		if strings.EqualFold(item.Name, label) {
			return true
		}
	}
	return false
}
func removeLabelFold(labels []string, remove string) []string {
	out := labels[:0]
	for _, label := range labels {
		if !strings.EqualFold(label, remove) {
			out = append(out, label)
		}
	}
	return DedupSorted(out)
}
func labelsToIssueLabels(labels []string) []IssueLabel {
	out := make([]IssueLabel, 0, len(labels))
	for _, label := range labels {
		out = append(out, IssueLabel{Name: label})
	}
	return out
}
func indexProjectItems(items []ProjectItem) map[string]ProjectItem {
	out := map[string]ProjectItem{}
	for _, item := range items {
		if item.ContentNodeID != "" {
			if item.FieldValues == nil {
				item.FieldValues = map[string]ProjectFieldValue{}
			}
			out[item.ContentNodeID] = item
		}
	}
	return out
}
