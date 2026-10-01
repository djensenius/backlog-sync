package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type App struct {
	Git     Git
	Backlog Backlog
	GitHub  GitHub
	Logf    func(string, ...any)
}

func (a App) Run(ctx context.Context, cfg Config) (Counters, error) {
	if a.Logf == nil {
		a.Logf = func(string, ...any) {}
	}
	resolved, err := CollectTasks(ctx, cfg.Root, a.Git, a.Backlog, a.Logf)
	if err != nil {
		return Counters{}, err
	}
	if err := a.validateProjectStatusOptions(ctx, cfg, resolved); err != nil {
		return Counters{}, err
	}
	issues, err := a.GitHub.ListIssues(ctx, cfg.Repo)
	if err != nil {
		return Counters{}, err
	}
	markerIssues, inboxIssues := IndexIssues(issues, a.Logf)
	issueByTaskID := map[string]Issue{}
	for id, issue := range markerIssues {
		issueByTaskID[id] = issue
	}

	var counters Counters
	if !cfg.NoInbox {
		imported, err := a.processInbox(ctx, cfg, issues, markerIssues, resolved)
		if err != nil {
			return counters, err
		}
		counters.Imported += imported
	}
	// Refresh the marker index if inbox labels were removed in a real run would be done next run;
	// this pass mirrors the tasks that existed at startup deterministically.
	_ = inboxIssues

	project, err := a.GitHub.ProjectInfo(ctx, cfg.ProjectOwner, cfg.ProjectNumber)
	if err != nil {
		return counters, err
	}
	projectItems, err := a.GitHub.ListProjectItems(ctx, project.ID)
	if err != nil {
		return counters, err
	}
	projectByContent := map[string]ProjectItem{}
	for _, item := range projectItems {
		if item.ContentNodeID != "" {
			projectByContent[item.ContentNodeID] = item
		}
	}
	for _, task := range resolved.Tasks {
		issue, exists := markerIssues[CanonicalTaskID(task.ID)]
		body := RenderIssueBody(task, issueByTaskID, resolved.ByID)
		labels := DesiredLabels(task.Labels, issue)
		if !exists {
			for _, label := range labels {
				if cfg.DryRun {
					a.Logf("would create label %q if missing", label)
				} else if err := a.GitHub.EnsureLabel(ctx, cfg.Repo, label); err != nil {
					return counters, err
				}
			}
			a.LogAction(cfg.DryRun, "create issue %s", IssueTitle(task))
			if !cfg.DryRun {
				created, err := a.GitHub.CreateIssue(ctx, cfg.Repo, IssueTitle(task), body, labels)
				if err != nil {
					return counters, err
				}
				issue = created
				markerIssues[CanonicalTaskID(task.ID)] = created
				issueByTaskID[CanonicalTaskID(task.ID)] = created
			}
			counters.Created++
			if task.Status == "Done" {
				state := "closed"
				reason := "completed"
				patch := IssuePatch{State: &state, StateReason: &reason}
				a.LogAction(cfg.DryRun, "close new issue %s as completed", CanonicalTaskID(task.ID))
				if !cfg.DryRun {
					updated, err := a.GitHub.UpdateIssue(ctx, cfg.Repo, issue.Number, patch)
					if err != nil {
						return counters, err
					}
					issue = updated
					markerIssues[CanonicalTaskID(task.ID)] = updated
					issueByTaskID[CanonicalTaskID(task.ID)] = updated
				}
				counters.Updated++
			}
		} else {
			patch, fields := DiffIssue(task, issue, body, labels)
			if len(fields) > 0 {
				for _, label := range labels {
					if cfg.DryRun {
						a.Logf("would create label %q if missing", label)
					} else if err := a.GitHub.EnsureLabel(ctx, cfg.Repo, label); err != nil {
						return counters, err
					}
				}
				a.LogAction(cfg.DryRun, "update issue #%d fields: %s", issue.Number, strings.Join(fields, ", "))
				if !cfg.DryRun {
					updated, err := a.GitHub.UpdateIssue(ctx, cfg.Repo, issue.Number, patch)
					if err != nil {
						return counters, err
					}
					issue = updated
					markerIssues[CanonicalTaskID(task.ID)] = updated
					issueByTaskID[CanonicalTaskID(task.ID)] = updated
				}
				counters.Updated++
			}
		}
		if issue.NodeID == "" {
			if cfg.DryRun && !exists {
				a.Logf("would add new issue for %s to project", CanonicalTaskID(task.ID))
				a.Logf("would set project status for new issue %s <unset>→%s", CanonicalTaskID(task.ID), task.Status)
				counters.ProjectAdded++
				counters.StatusChanges++
			}
			continue
		}
		item, ok := projectByContent[issue.NodeID]
		if !ok {
			a.LogAction(cfg.DryRun, "add issue #%d to project", issue.Number)
			if !cfg.DryRun {
				added, err := a.GitHub.AddProjectItem(ctx, project.ID, issue.NodeID)
				if err != nil {
					return counters, err
				}
				item = added
				item.ContentNodeID = issue.NodeID
				projectByContent[issue.NodeID] = item
			}
			counters.ProjectAdded++
		}
		optionID := project.StatusOptionID[task.Status]
		if item.ID != "" && item.StatusOptionID != optionID {
			old := item.Status
			if old == "" {
				old = "<unset>"
			}
			a.LogAction(cfg.DryRun, "set project status for issue #%d %s→%s", issue.Number, old, task.Status)
			if !cfg.DryRun {
				if err := a.GitHub.UpdateProjectStatus(ctx, project.ID, item.ID, project.StatusFieldID, optionID); err != nil {
					return counters, err
				}
			}
			counters.StatusChanges++
		}
	}
	a.Logf("sync complete: %d created, %d updated, %d status changes, %d imported", counters.Created, counters.Updated, counters.StatusChanges, counters.Imported)
	return counters, nil
}

func (a App) validateProjectStatusOptions(ctx context.Context, cfg Config, resolved ResolvedTasks) error {
	statuses, err := a.Backlog.Statuses(ctx, cfg.Root)
	if err != nil {
		return err
	}
	project, err := a.GitHub.ProjectInfo(ctx, cfg.ProjectOwner, cfg.ProjectNumber)
	if err != nil {
		return err
	}
	missing := []string{}
	for _, status := range statuses {
		if project.StatusOptionID[status] == "" {
			missing = append(missing, status)
		}
	}
	extra := []string{}
	allowed := map[string]bool{}
	for _, status := range statuses {
		allowed[status] = true
	}
	for option := range project.StatusOptionID {
		if !allowed[option] {
			extra = append(extra, option)
		}
	}
	if len(missing) > 0 || len(extra) > 0 {
		sort.Strings(missing)
		sort.Strings(extra)
		return fmt.Errorf("project Status options do not match backlog statuses; missing=%v extra=%v", missing, extra)
	}
	_ = resolved
	return nil
}

func IndexIssues(issues []Issue, logf func(string, ...any)) (map[string]Issue, []Issue) {
	byMarker := map[string]Issue{}
	var inbox []Issue
	for _, issue := range issues {
		labels := LabelsOf(issue)
		for _, label := range labels {
			if label == "inbox" && issue.State == "open" {
				inbox = append(inbox, issue)
			}
		}
		id, ok := ParseMarker(issue.Body)
		if !ok {
			continue
		}
		existing, dup := byMarker[id]
		if !dup || issue.Number < existing.Number {
			if dup && logf != nil {
				logf("warning: duplicate marker %s on #%d and #%d; using #%d", id, existing.Number, issue.Number, min(existing.Number, issue.Number))
			}
			byMarker[id] = issue
		} else if logf != nil {
			logf("warning: duplicate marker %s on #%d and #%d; using #%d", id, existing.Number, issue.Number, existing.Number)
		}
	}
	return byMarker, inbox
}

func DesiredLabels(taskLabels []string, issue Issue) []string {
	labels := append([]string(nil), taskLabels...)
	for _, label := range issue.Labels {
		if label.Name == "inbox" {
			labels = append(labels, "inbox")
		}
	}
	return DedupSorted(labels)
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
	if !EqualStringSlices(LabelsOf(issue), labels) {
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
		} else {
			patch.StateReason = nil
		}
	}
	return patch, fields
}

func (a App) processInbox(ctx context.Context, cfg Config, issues []Issue, markerIssues map[string]Issue, resolved ResolvedTasks) (int, error) {
	clean, reason, err := a.Git.RootBranchClean(ctx, cfg.Root)
	if err != nil {
		return 0, err
	}
	imported := 0
	for _, issue := range issues {
		if issue.PullRequest != nil || issue.State != "open" || !hasLabel(issue, "inbox") {
			continue
		}
		if id, ok := ParseMarker(issue.Body); ok {
			labels := removeLabel(LabelsOf(issue), "inbox")
			patch := IssuePatch{Labels: &labels}
			a.LogAction(cfg.DryRun, "remove inbox label from mirrored issue #%d (%s)", issue.Number, id)
			if !cfg.DryRun {
				if _, err := a.GitHub.UpdateIssue(ctx, cfg.Repo, issue.Number, patch); err != nil {
					return imported, err
				}
			}
			continue
		}
		if !clean {
			a.Logf("skip inbox issue #%d: %s", issue.Number, reason)
			continue
		}
		importedMarker := fmt.Sprintf("Imported from GitHub issue #%d:", issue.Number)
		var taskID string
		for _, task := range resolved.Tasks {
			if strings.Contains(task.Description, importedMarker) {
				taskID = task.ID
				break
			}
		}
		if taskID == "" {
			description := strings.TrimSpace(issue.Body) + fmt.Sprintf("\n\nImported from GitHub issue #%d: %s", issue.Number, issue.HTMLURL)
			if cfg.DryRun {
				a.Logf("would create task from #%d", issue.Number)
				taskID = "TASK-DRY-RUN"
			} else {
				id, err := a.Backlog.CreateTask(ctx, cfg.Root, issue.Title, description, []string{"inbox"})
				if err != nil {
					return imported, err
				}
				taskID = id
				imported++
			}
		} else {
			a.Logf("reuse existing imported task %s for issue #%d", CanonicalTaskID(taskID), issue.Number)
		}
		if cfg.DryRun {
			continue
		}
		body := MarkerFor(taskID) + "\n" + issue.Body
		title := fmt.Sprintf("%s: %s", CanonicalTaskID(taskID), issue.Title)
		labels := removeLabel(LabelsOf(issue), "inbox")
		patch := IssuePatch{Title: &title, Body: &body, Labels: &labels}
		if _, err := a.GitHub.UpdateIssue(ctx, cfg.Repo, issue.Number, patch); err != nil {
			return imported, err
		}
	}
	return imported, nil
}

func (a App) LogAction(dryRun bool, format string, args ...any) {
	prefix := ""
	if dryRun {
		prefix = "would "
	}
	message := fmt.Sprintf(format, args...)
	if dryRun && strings.HasPrefix(message, "would ") {
		a.Logf("%s", message)
		return
	}
	a.Logf("%s%s", prefix, message)
}

func hasLabel(issue Issue, label string) bool {
	for _, item := range issue.Labels {
		if item.Name == label {
			return true
		}
	}
	return false
}

func removeLabel(labels []string, remove string) []string {
	out := labels[:0]
	for _, label := range labels {
		if label != remove {
			out = append(out, label)
		}
	}
	return DedupSorted(out)
}
