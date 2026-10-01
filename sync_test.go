package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func tp(s string) *time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return &t
}
func sp(s string) *string { return &s }

func testConfig(root string) Config {
	return Config{Root: root, ProjectOwner: "owner", ProjectOwnerType: "user", ProjectNumber: 8, DefaultRepo: "owner/repo", MainBranch: "main", Inbox: InboxConfig{Enabled: true, Label: "inbox"}, Labels: LabelConfig{Managed: []string{"*"}}, SubIssues: true, TaskPrefix: "task"}.Normalized()
}

func TestResolveTaskCopyAndNestedBranchOwnership(t *testing.T) {
	statusRank := map[string]int{"To Do": 0, "In Progress": 1, "Done": 2}
	root := "/repo"
	copies := []TaskCopy{{Task: Task{ID: "TASK-1.2.7", Status: "Done", UpdatedAt: tp("2026-10-01T10:00:00Z")}, Worktree: Worktree{Path: root, Branch: "main", IsRoot: true}}, {Task: Task{ID: "TASK-1.2.7", Status: "To Do", UpdatedAt: tp("2026-10-01T09:00:00Z")}, Worktree: Worktree{Path: "/repo-task", Branch: "TASK-1.2.7-fix"}}}
	got := ResolveTaskCopy("task-1.2.7", copies, statusRank, root)
	if got.Worktree.Branch != "TASK-1.2.7-fix" {
		t.Fatalf("wanted exact nested branch owner, got %s", got.Worktree.Branch)
	}
	if !BranchOwnsTask("task-1.2.7", "task-1.2.7") || !BranchOwnsTask("task-1.2.7", "TASK-1.2.7-foo") {
		t.Fatalf("nested branch owner should match exact or id-dash")
	}
	if BranchOwnsTask("task-1.2.7", "task-1.2-foo") || BranchOwnsTask("task-1.2", "task-1.2.7-foo") {
		t.Fatalf("nested branch ownership matched a sibling/descendant incorrectly")
	}
}

func TestRenderIssueBodyGolden(t *testing.T) {
	task := sampleTask()
	parent := Task{ID: "TASK-1", Title: "Parent"}
	body := RenderIssueBodyWithOptions(task, RenderOptions{IssueByTaskID: map[string]Issue{"task-1": {HTMLURL: "https://example.test/1"}}, TaskByID: map[string]Task{"task-1": parent}, Milestones: map[string]string{"m-0": "v1 release"}})
	want := `<!-- backlog:task-16 -->
Mirrored one-way from Backlog.md — edits here are overwritten. Change it with the backlog CLI.

Status: In Progress
Branch: task-16-backlog-sync
Project: apple
Milestone: v1 release
Priority: High
Labels: ci, tooling
Assignees: @pi-worker
Parent: [task-1: Parent](https://example.test/1)
Depends on:
- [task-1: Parent](https://example.test/1)
Subtasks:
- task-16.1: Child

## Description
Build a sync.

## Acceptance criteria
- [x] First
- [ ] Second

## Implementation plan
Plan it.

## Implementation notes
Built it.

## Final summary
Finished.
`
	if body != want {
		t.Fatalf("body mismatch\n--- got ---\n%s\n--- want ---\n%s", body, want)
	}
	if body2 := RenderIssueBodyWithOptions(task, RenderOptions{IssueByTaskID: map[string]Issue{"task-1": {HTMLURL: "https://example.test/1"}}, TaskByID: map[string]Task{"task-1": parent}, Milestones: map[string]string{"m-0": "v1 release"}}); body2 != body {
		t.Fatalf("rendering is not deterministic")
	}
}

func TestParseMarkerStrictFirstLineAndDotted(t *testing.T) {
	cases := []struct {
		name, body, want string
		ok               bool
	}{{"first line", "<!-- backlog:task-1.6 -->\nbody", "task-1.6", true}, {"nested", "<!-- backlog:task-1.2.7 -->\nbody", "task-1.2.7", true}, {"uppercase ignored", "<!-- backlog:TASK-1 -->\nbody", "", false}, {"not first line", "```\n<!-- backlog:task-1 -->\n```", "", false}, {"loose ignored", " <!-- backlog:task-1 -->", "", false}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseMarker(tc.body)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("ParseMarker()=(%q,%v), want (%q,%v)", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestDesiredLabelsManagedCaseInsensitiveAndInboxPreserved(t *testing.T) {
	cfg := testConfig("/repo")
	cfg.Labels = LabelConfig{Managed: []string{"backlog", "type:*", "priority:*", "CI"}, AddAlways: []string{"backlog"}, PriorityPrefix: "priority:"}
	cfg.Inbox.Label = "inbox"
	task := Task{Labels: []string{"CI", "type:Bug", "unmanaged", "inbox"}, Priority: sp("High")}
	issue := Issue{Labels: []IssueLabel{{Name: "ci"}, {Name: "type:old"}, {Name: "external"}, {Name: "inbox"}}}
	got := DesiredLabels(task, issue, cfg)
	want := []string{"backlog", "CI", "external", "inbox", "priority:high", "type:Bug"}
	if !EqualStringSlicesFold(got, want, true) {
		t.Fatalf("labels=%v want %v", got, want)
	}
	patch, fields := DiffIssue(task, Issue{Title: IssueTitle(task), Body: "body", State: "open", Labels: []IssueLabel{{Name: "ci"}, {Name: "BACKLOG"}, {Name: "external"}, {Name: "inbox"}, {Name: "priority:HIGH"}, {Name: "type:Bug"}}}, "body", got)
	if contains(fields, "labels") || patch.Labels != nil {
		t.Fatalf("case-only label differences should be no-op: fields=%v", fields)
	}
}

func TestAppMultiRepoAdoptionProjectMoveAndTwoRunNoop(t *testing.T) {
	ctx := context.Background()
	root := tempRoot(t)
	task := sampleTask()
	task.ParentTaskID = nil
	task.Project = sp("apple")
	task.References = []string{"https://github.com/owner/apple/issues/7"}
	cfg := testConfig(root)
	cfg.Repos = map[string]string{"apple": "owner/apple"}
	cfg.DefaultRepo = "owner/meta"
	cfg.AdoptReferencedIssues = true
	cfg.Labels = LabelConfig{Managed: []string{"*"}, AddAlways: []string{"backlog"}}
	bl := newFakeBacklog(root, task)
	gh := basicGH(map[string][]Issue{"owner/apple": {{Number: 7, DatabaseID: 700, NodeID: "I_7", HTMLURL: "https://github.com/owner/apple/issues/7", Title: "Old", Body: "old", State: "open", Repo: "owner/apple"}}, "owner/meta": {}})
	app := App{Git: fakeGit{worktrees: []Worktree{{Path: root, Branch: "main", IsRoot: true}}, rootClean: true}, Backlog: bl, GitHub: gh, Logf: func(string, ...any) {}}
	c, err := app.Run(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if c.Created != 0 || c.Updated == 0 || len(gh.createdIssues) != 0 {
		t.Fatalf("adoption should update existing issue without creating duplicate: counters=%+v created=%d", c, len(gh.createdIssues))
	}
	// Re-run over the mutated fake GitHub state: second pass should be a full no-op.
	c, err = app.Run(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if c.Created != 0 || c.Updated != 0 || c.StatusChanges != 0 || c.ProjectAdded != 0 {
		t.Fatalf("second run should be no-op, counters=%+v", c)
	}
	// Moving the task's project should keep the existing issue in place and not create a duplicate.
	task.Project = sp("unknown")
	bl.tasksByDir[root] = []Task{task}
	c, err = app.Run(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if c.Created != 0 || len(gh.createdIssues) != 0 {
		t.Fatalf("project move should not create duplicate, counters=%+v created=%d", c, len(gh.createdIssues))
	}
}

func TestInboxImportReplayIndexesIssueInSameRun(t *testing.T) {
	ctx := context.Background()
	root := tempRoot(t)
	task := sampleTask()
	task.ParentTaskID = nil
	replayed := task
	replayed.ID = "TASK-88"
	replayed.References = []string{"https://github.com/owner/repo/issues/44"}
	inboxIssue := Issue{Number: 44, DatabaseID: 440, NodeID: "I_44", HTMLURL: "https://github.com/owner/repo/issues/44", Title: "New idea", Body: "Please do it", State: "open", Repo: "owner/repo", Labels: []IssueLabel{{Name: "inbox"}}}
	_ = task
	bl := newFakeBacklog(root, replayed)
	gh := basicGH(map[string][]Issue{"owner/repo": {inboxIssue}})
	app := App{Git: fakeGit{worktrees: []Worktree{{Path: root, Branch: "main", IsRoot: true}}, rootClean: true}, Backlog: bl, GitHub: gh, Logf: func(string, ...any) {}}
	c, err := app.Run(ctx, testConfig(root))
	if err != nil {
		t.Fatal(err)
	}
	if c.Imported != 0 || len(bl.created) != 0 || len(gh.createdIssues) != 0 {
		t.Fatalf("expected replay reuse without duplicate create, counters=%+v tasks=%d issues=%d", c, len(bl.created), len(gh.createdIssues))
	}
	issue := gh.issues["owner/repo"][0]
	if !strings.HasPrefix(issue.Title, "task-88:") || !strings.HasPrefix(issue.Body, "<!-- backlog:task-88 -->") || hasLabelFold(issue, "inbox") {
		t.Fatalf("inbox issue not marked/label removed: %+v", issue)
	}
}

func TestMultiRepoInboxCreatesWithProjectAndReferenceGuard(t *testing.T) {
	ctx := context.Background()
	root := tempRoot(t)
	existing := sampleTask()
	existing.ParentTaskID = nil
	existing.References = []string{"https://github.com/owner/apple/issues/6"}
	issue6 := Issue{Number: 6, NodeID: "I_6", HTMLURL: "https://github.com/owner/apple/issues/6", Title: "Existing", Body: "old", State: "open", Repo: "owner/apple", Labels: []IssueLabel{{Name: "inbox"}}}
	issue7 := Issue{Number: 7, NodeID: "I_7", HTMLURL: "https://github.com/owner/apple/issues/7", Title: "New", Body: "new", State: "open", Repo: "owner/apple", Labels: []IssueLabel{{Name: "inbox"}}}
	bl := newFakeBacklog(root, existing)
	bl.createdID = "TASK-99"
	gh := basicGH(map[string][]Issue{"owner/repo": {}, "owner/apple": {issue6, issue7}})
	cfg := testConfig(root)
	cfg.Repos = map[string]string{"apple": "owner/apple"}
	cfg.Inbox.Push = true
	app := App{Git: fakeGit{worktrees: []Worktree{{Path: root, Branch: "main", IsRoot: true}}, rootClean: true}, Backlog: bl, GitHub: gh, Logf: func(string, ...any) {}}
	c, err := app.Run(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if c.Imported != 1 || len(bl.created) != 1 || bl.created[0].Project == nil || *bl.created[0].Project != "apple" || len(bl.created[0].References) != 1 {
		t.Fatalf("bad inbox import counters=%+v created=%+v", c, bl.created)
	}
	if !bl.pushed {
		t.Fatalf("expected inbox push")
	}
}

func TestProjectFieldsMilestoneParserAndSubIssueWarnOnce(t *testing.T) {
	got := ParseMilestones("  m-0: M1: Night of the Zealot on Apple (0/3 done)\n")
	if got["m-0"] != "M1: Night of the Zealot on Apple" {
		t.Fatalf("milestone parse=%q", got["m-0"])
	}
	ctx := context.Background()
	root := tempRoot(t)
	parent := sampleTask()
	parent.ID = "TASK-1"
	parent.ParentTaskID = nil
	child := sampleTask()
	child.ID = "TASK-1.1"
	child.ParentTaskID = sp("TASK-1")
	cfg := testConfig(root)
	cfg.Fields = FieldConfig{Priority: "Priority", Milestone: "Backlog milestone", Area: "Area", TaskID: "Task ID", Branch: "Branch"}
	cfg.Labels = LabelConfig{Managed: []string{"*"}}
	bl := newFakeBacklog(root, parent, child)
	gh := basicGH(map[string][]Issue{"owner/repo": {{Number: 1, DatabaseID: 1, NodeID: "I_PARENT", HTMLURL: "u1", Title: IssueTitle(parent), Body: RenderIssueBody(parent, nil, nil), State: "open", Repo: "owner/repo"}, {Number: 2, DatabaseID: 2, NodeID: "I_CHILD", HTMLURL: "u2", Title: IssueTitle(child), Body: RenderIssueBody(child, nil, nil), State: "open", Repo: "owner/repo"}}})
	gh.project.Fields["Priority"] = ProjectField{ID: "PF", Options: map[string]string{"High": "PH"}}
	gh.project.Fields["Backlog milestone"] = ProjectField{ID: "MF", Options: map[string]string{"v1 release": "M0"}}
	gh.project.Fields["Area"] = ProjectField{ID: "AF", Options: map[string]string{"apple": "AA"}}
	gh.project.Fields["Task ID"] = ProjectField{ID: "TF"}
	gh.project.Fields["Branch"] = ProjectField{ID: "BF"}
	gh.subIssueErr = errors.New("unsupported")
	app := App{Git: fakeGit{worktrees: []Worktree{{Path: root, Branch: "main", IsRoot: true}}, rootClean: true}, Backlog: bl, GitHub: gh, Logf: func(string, ...any) {}}
	_, err := app.Run(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if gh.fieldUpdates == 0 {
		t.Fatalf("expected project field updates")
	}
	if gh.addSubIssueCalls != 1 {
		t.Fatalf("expected one sub-issue attempt, got %d", gh.addSubIssueCalls)
	}
}

func TestAllowlistRefusesUnconfiguredRepo(t *testing.T) {
	gh := ExecGitHub{Runner: fakeRunner{}, AllowedRepos: []string{"djensenius/ArkhamHorror"}}
	if _, err := gh.ListIssues(context.Background(), "halogenandtoast/ArkhamHorror"); err == nil || !strings.Contains(err.Error(), "refusing") {
		t.Fatalf("expected allowlist refusal, got %v", err)
	}
}

func TestPaginationTotalLockAndEnvScrub(t *testing.T) {
	if !contains(scrubBacklogEnv([]string{"BACKLOG_CWD=/tmp/x", "BACKLOG_ROOT=/tmp/y", "PATH=/bin"}), "PATH=/bin") {
		t.Fatalf("PATH should remain")
	}
	for _, env := range scrubBacklogEnv([]string{"BACKLOG_CWD=/tmp/x", "BACKLOG_ROOT=/tmp/y"}) {
		if strings.HasPrefix(env, "BACKLOG_") {
			t.Fatalf("BACKLOG env not scrubbed: %v", env)
		}
	}
	root := tempRoot(t)
	bl := newFakeBacklog(root, sampleTask())
	bl.badTotal = true
	_, err := listAllTasks(context.Background(), bl, root)
	if err == nil || !strings.Contains(err.Error(), "total") {
		t.Fatalf("expected total mismatch, got %v", err)
	}
	cfg := testConfig(root)
	cfg.LockFile = filepath.Join(root, "sync.lock")
	unlock, err := acquireRunLock(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	unlock2, err := acquireRunLock(cfg)
	if !errors.Is(err, errLocked) {
		if unlock2 != nil {
			unlock2()
		}
		t.Fatalf("expected locked, got %v", err)
	}
}

func TestAbortOnEmptyMainTasks(t *testing.T) {
	root := tempRoot(t)
	bl := &fakeBacklog{statuses: []string{"To Do", "In Progress", "Done"}, tasksByDir: map[string][]Task{root: {}}}
	gh := basicGH(map[string][]Issue{"owner/repo": {}})
	app := App{Git: fakeGit{worktrees: []Worktree{{Path: root, Branch: "main", IsRoot: true}}}, Backlog: bl, GitHub: gh, Logf: func(string, ...any) {}}
	_, err := app.Run(context.Background(), testConfig(root))
	if err == nil || !strings.Contains(err.Error(), "returned 0 tasks") {
		t.Fatalf("expected empty-main safety error, got %v", err)
	}
	if gh.listIssuesCalls != 0 {
		t.Fatalf("GitHub was touched despite safety abort")
	}
}

func tempRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "backlog"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func sampleTask() Task {
	return Task{ID: "TASK-16", Title: "Backlog sync", Status: "In Progress", Project: sp("apple"), Priority: sp("High"), Labels: []string{"tooling", "ci"}, Milestone: sp("m-0"), ParentTaskID: sp("TASK-1"), Assignees: []string{"@pi-worker"}, CreatedAt: tp("2026-10-01T02:33:00Z"), UpdatedAt: tp("2026-10-01T02:35:00Z"), Description: "Build a sync.", Dependencies: []string{"TASK-1"}, Subtasks: []TaskRef{{ID: "TASK-16.1", Title: "Child"}}, AcceptanceCriteria: []AcceptanceCriterion{{Index: 1, Text: "First", Checked: true}, {Index: 2, Text: "Second", Checked: false}}, ImplementationPlan: "Plan it.", ImplementationNotes: "Built it.", FinalSummary: sp("Finished."), Branch: "task-16-backlog-sync"}
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

type fakeRunner struct{}

func (fakeRunner) Run(context.Context, string, string, []string, []byte) ([]byte, error) {
	return nil, errors.New("should not run")
}

type fakeGit struct {
	worktrees  []Worktree
	rootClean  bool
	rootReason string
}

func (g fakeGit) Worktrees(context.Context, string) ([]Worktree, error) { return g.worktrees, nil }
func (g fakeGit) RootBranchClean(context.Context, string, string) (bool, string, error) {
	return g.rootClean, g.rootReason, nil
}

type fakeBacklog struct {
	statuses   []string
	tasksByDir map[string][]Task
	createdID  string
	created    []Task
	pushed     bool
	badTotal   bool
}

func newFakeBacklog(root string, tasks ...Task) *fakeBacklog {
	return &fakeBacklog{statuses: []string{"To Do", "In Progress", "Done"}, tasksByDir: map[string][]Task{root: tasks}, createdID: "TASK-99"}
}
func (b *fakeBacklog) ListTasks(_ context.Context, dir string, maxCount, skip int) (TaskListResponse, error) {
	tasks := b.tasksByDir[dir]
	if skip > len(tasks) {
		skip = len(tasks)
	}
	end := skip + maxCount
	if end > len(tasks) {
		end = len(tasks)
	}
	summaries := make([]TaskSummary, 0, end-skip)
	for _, task := range tasks[skip:end] {
		summaries = append(summaries, TaskSummary{ID: task.ID, Title: task.Title, Status: task.Status, Labels: task.Labels, Milestone: task.Milestone, ParentTaskID: task.ParentTaskID, Assignees: task.Assignees, CreatedAt: task.CreatedAt, UpdatedAt: task.UpdatedAt})
	}
	total := len(tasks)
	if b.badTotal {
		total++
	}
	var next *int
	if end < len(tasks) {
		n := end
		next = &n
	}
	return TaskListResponse{Tasks: summaries, Total: total, NextSkip: next}, nil
}
func (b *fakeBacklog) ViewTask(_ context.Context, dir, id string) (TaskViewResponse, error) {
	for _, task := range b.tasksByDir[dir] {
		if CanonicalTaskID(task.ID) == CanonicalTaskID(id) {
			return TaskViewResponse{Task: task}, nil
		}
	}
	return TaskViewResponse{}, nil
}
func (b *fakeBacklog) Statuses(context.Context, string) ([]string, error) { return b.statuses, nil }
func (b *fakeBacklog) TaskPrefix(context.Context, string) (string, error) { return "task", nil }
func (b *fakeBacklog) Milestones(context.Context, string) (map[string]string, error) {
	return map[string]string{"m-0": "v1 release"}, nil
}
func (b *fakeBacklog) CreateTask(_ context.Context, _ string, in CreateTaskInput) (string, error) {
	p := in.Project
	task := Task{ID: b.createdID, Title: in.Title, Description: in.Description, Labels: in.Labels, Status: "To Do", Project: &p, References: in.References}
	b.created = append(b.created, task)
	b.tasksByDir[firstDir(b.tasksByDir)] = append(b.tasksByDir[firstDir(b.tasksByDir)], task)
	return b.createdID, nil
}
func (b *fakeBacklog) Push(context.Context, string, string) error { b.pushed = true; return nil }
func firstDir(m map[string][]Task) string {
	for k := range m {
		return k
	}
	return ""
}

type fakeGitHub struct {
	issues                   map[string][]Issue
	project                  ProjectInfo
	items                    []ProjectItem
	listIssuesCalls          int
	updatedIssues            []IssuePatch
	createdIssues            []Issue
	addProjectItemCalls      int
	updateProjectStatusCalls int
	fieldUpdates             int
	parents                  map[string]string
	addSubIssueCalls         int
	subIssueErr              error
}

func basicGH(issues map[string][]Issue) *fakeGitHub {
	gh := &fakeGitHub{issues: issues, project: ProjectInfo{ID: "P", StatusFieldID: "F", StatusOptionID: map[string]string{"To Do": "O1", "In Progress": "O2", "Done": "O3"}, Fields: map[string]ProjectField{"Status": {ID: "F", Options: map[string]string{"To Do": "O1", "In Progress": "O2", "Done": "O3"}}}}, parents: map[string]string{}}
	for _, repoIssues := range issues {
		for _, issue := range repoIssues {
			gh.items = append(gh.items, ProjectItem{ID: "PVTI_" + issue.NodeID, ContentNodeID: issue.NodeID, Status: "In Progress", StatusOptionID: "O2", FieldValues: map[string]ProjectFieldValue{}})
		}
	}
	return gh
}
func (g *fakeGitHub) ListIssues(_ context.Context, repo string) ([]Issue, error) {
	g.listIssuesCalls++
	out := append([]Issue(nil), g.issues[repo]...)
	for i := range out {
		out[i].Repo = repo
	}
	return out, nil
}
func (g *fakeGitHub) CreateIssue(_ context.Context, repo string, title string, body string, labels []string) (Issue, error) {
	issue := Issue{Number: 100 + len(g.createdIssues), DatabaseID: int64(100 + len(g.createdIssues)), NodeID: "I_NEW_" + title, HTMLURL: "https://github.com/" + repo + "/issues/100", Title: title, Body: body, State: "open", Repo: repo, Labels: labelsToIssueLabels(labels)}
	g.createdIssues = append(g.createdIssues, issue)
	g.issues[repo] = append(g.issues[repo], issue)
	g.items = append(g.items, ProjectItem{ID: "PVTI_" + issue.NodeID, ContentNodeID: issue.NodeID, Status: "", FieldValues: map[string]ProjectFieldValue{}})
	return issue, nil
}
func (g *fakeGitHub) UpdateIssue(_ context.Context, repo string, number int, patch IssuePatch) (Issue, error) {
	g.updatedIssues = append(g.updatedIssues, patch)
	for i, issue := range g.issues[repo] {
		if issue.Number == number {
			if patch.Title != nil {
				issue.Title = *patch.Title
			}
			if patch.Body != nil {
				issue.Body = *patch.Body
			}
			if patch.Labels != nil {
				issue.Labels = labelsToIssueLabels(*patch.Labels)
			}
			if patch.State != nil {
				issue.State = *patch.State
			}
			issue.Repo = repo
			g.issues[repo][i] = issue
			return issue, nil
		}
	}
	return Issue{Number: number, Repo: repo}, nil
}
func (g *fakeGitHub) EnsureLabel(context.Context, string, string) error { return nil }
func (g *fakeGitHub) ProjectInfo(context.Context, string, string, int) (ProjectInfo, error) {
	return g.project, nil
}
func (g *fakeGitHub) ListProjectItems(context.Context, string) ([]ProjectItem, error) {
	return append([]ProjectItem(nil), g.items...), nil
}
func (g *fakeGitHub) AddProjectItem(_ context.Context, _ string, contentNodeID string) (ProjectItem, error) {
	g.addProjectItemCalls++
	item := ProjectItem{ID: "PVTI_NEW", ContentNodeID: contentNodeID, FieldValues: map[string]ProjectFieldValue{}}
	g.items = append(g.items, item)
	return item, nil
}
func (g *fakeGitHub) UpdateProjectStatus(context.Context, string, string, string, string) error {
	g.updateProjectStatusCalls++
	return nil
}
func (g *fakeGitHub) UpdateProjectSingleSelect(context.Context, string, string, string, string) error {
	g.fieldUpdates++
	return nil
}
func (g *fakeGitHub) UpdateProjectText(context.Context, string, string, string, string) error {
	g.fieldUpdates++
	return nil
}
func (g *fakeGitHub) ClearProjectField(context.Context, string, string, string) error {
	g.fieldUpdates++
	return nil
}
func (g *fakeGitHub) IssueParent(_ context.Context, issueNodeID string) (string, error) {
	return g.parents[issueNodeID], nil
}
func (g *fakeGitHub) AddSubIssue(context.Context, string, int, int64) error {
	g.addSubIssueCalls++
	return g.subIssueErr
}
func (g *fakeGitHub) RemoveSubIssue(context.Context, string, string) error { return nil }
