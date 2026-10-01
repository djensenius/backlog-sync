package main

import (
	"context"
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

func TestResolveTaskCopy(t *testing.T) {
	statusRank := map[string]int{"To Do": 0, "In Progress": 1, "Done": 2}
	root := "/repo"
	t.Run("branch-name owner wins", func(t *testing.T) {
		copies := []TaskCopy{
			{Task: Task{ID: "TASK-2", Status: "Done", UpdatedAt: tp("2026-10-01T10:00:00Z")}, Worktree: Worktree{Path: root, Branch: "main", IsRoot: true}},
			{Task: Task{ID: "TASK-2", Status: "To Do", UpdatedAt: tp("2026-10-01T09:00:00Z")}, Worktree: Worktree{Path: "/repo-task-2", Branch: "task-2-json-schemas"}},
		}
		got := ResolveTaskCopy("task-2", copies, statusRank, root)
		if got.Worktree.Branch != "task-2-json-schemas" {
			t.Fatalf("wanted branch owner, got %s", got.Worktree.Branch)
		}
	})
	t.Run("latest updatedAt wins", func(t *testing.T) {
		copies := []TaskCopy{
			{Task: Task{ID: "TASK-3", Status: "To Do", UpdatedAt: tp("2026-10-01T09:00:00Z")}, Worktree: Worktree{Path: root, Branch: "main", IsRoot: true}},
			{Task: Task{ID: "TASK-3", Status: "To Do", UpdatedAt: tp("2026-10-01T10:00:00Z")}, Worktree: Worktree{Path: "/other", Branch: "feature"}},
		}
		got := ResolveTaskCopy("task-3", copies, statusRank, root)
		if got.Worktree.Path != "/other" {
			t.Fatalf("wanted latest worktree, got %s", got.Worktree.Path)
		}
	})
	t.Run("tie by status rank then main fallback", func(t *testing.T) {
		copies := []TaskCopy{
			{Task: Task{ID: "TASK-4", Status: "In Progress", UpdatedAt: tp("2026-10-01T09:00:00Z")}, Worktree: Worktree{Path: root, Branch: "main", IsRoot: true}},
			{Task: Task{ID: "TASK-4", Status: "Done", UpdatedAt: tp("2026-10-01T09:00:00Z")}, Worktree: Worktree{Path: "/other", Branch: "feature"}},
		}
		got := ResolveTaskCopy("task-4", copies, statusRank, root)
		if got.Task.Status != "Done" {
			t.Fatalf("wanted higher status rank, got %s", got.Task.Status)
		}
		copies = []TaskCopy{
			{Task: Task{ID: "TASK-4", Status: "In Progress", UpdatedAt: tp("2026-10-01T09:00:00Z")}, Worktree: Worktree{Path: root, Branch: "main", IsRoot: true}},
			{Task: Task{ID: "TASK-4", Status: "In Progress", UpdatedAt: tp("2026-10-01T09:00:00Z")}, Worktree: Worktree{Path: "/other", Branch: "feature"}},
		}
		got = ResolveTaskCopy("task-4", copies, statusRank, root)
		if !got.Worktree.IsRoot {
			t.Fatalf("wanted main fallback, got %s", got.Worktree.Path)
		}
	})
}

func TestRenderIssueBodyGolden(t *testing.T) {
	task := sampleTask()
	parent := Task{ID: "TASK-1", Title: "Parent"}
	body := RenderIssueBody(task, map[string]Issue{"task-1": {HTMLURL: "https://example.test/1"}}, map[string]Task{"task-1": parent})
	want := `<!-- backlog:task-16 -->
Mirrored one-way from Backlog.md — edits here are overwritten. Change it with the backlog CLI.

Status: In Progress
Branch: task-16-backlog-sync
Parent: [task-1: Parent](https://example.test/1)
Subtasks:
- task-16.1: Child
Depends on:
- [task-1: Parent](https://example.test/1)
Milestone: m-0
Assignees: @pi-worker
Labels: ci, tooling

## Description
Build a sync.

## Acceptance criteria
- [x] First
- [ ] Second

## Final summary
Finished.
`
	if body != want {
		t.Fatalf("body mismatch\n--- got ---\n%s\n--- want ---\n%s", body, want)
	}
	if body2 := RenderIssueBody(task, map[string]Issue{"task-1": {HTMLURL: "https://example.test/1"}}, map[string]Task{"task-1": parent}); body2 != body {
		t.Fatalf("rendering is not deterministic")
	}
}

func TestParseMarkerStrictFirstLine(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
		ok   bool
	}{
		{"first line", "<!-- backlog:task-1.6 -->\nbody", "task-1.6", true},
		{"uppercase ignored", "<!-- backlog:TASK-1 -->\nbody", "", false},
		{"not first line", "```\n<!-- backlog:task-1 -->\n```", "", false},
		{"loose ignored", " <!-- backlog:task-1 -->", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseMarker(tc.body)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("ParseMarker()=(%q,%v), want (%q,%v)", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestDiffIssueIdempotentCloseAndReopen(t *testing.T) {
	task := sampleTask()
	body := RenderIssueBody(task, nil, map[string]Task{CanonicalTaskID(task.ID): task})
	issue := Issue{Title: IssueTitle(task), Body: body, State: "open", Labels: []IssueLabel{{Name: "ci"}, {Name: "tooling"}}}
	_, fields := DiffIssue(task, issue, body, []string{"tooling", "ci"})
	if len(fields) != 0 {
		t.Fatalf("second run should be no-op, got fields %v", fields)
	}
	done := task
	done.Status = "Done"
	patch, fields := DiffIssue(done, issue, body, []string{"ci", "tooling"})
	if !contains(fields, "state") || patch.State == nil || *patch.State != "closed" || patch.StateReason == nil || *patch.StateReason != "completed" {
		t.Fatalf("Done should close as completed, fields=%v patch=%+v", fields, patch)
	}
	issue.State = "closed"
	patch, fields = DiffIssue(task, issue, body, []string{"ci", "tooling"})
	if !contains(fields, "state") || patch.State == nil || *patch.State != "open" {
		t.Fatalf("non-Done should reopen, fields=%v patch=%+v", fields, patch)
	}
}

func TestAppNoopAndProjectStatusDiff(t *testing.T) {
	ctx := context.Background()
	root := tempRoot(t)
	task := sampleTask()
	task.Branch = "main"
	body := RenderIssueBody(task, nil, map[string]Task{CanonicalTaskID(task.ID): task})
	mkApp := func(status string) (*fakeGitHub, App) {
		bl := newFakeBacklog(root, task)
		gh := &fakeGitHub{
			issues:  []Issue{{Number: 7, NodeID: "I_7", HTMLURL: "https://example.test/7", Title: IssueTitle(task), Body: body, State: "open", Labels: []IssueLabel{{Name: "ci"}, {Name: "tooling"}}}},
			project: ProjectInfo{ID: "P", StatusFieldID: "F", StatusOptionID: map[string]string{"To Do": "O1", "In Progress": "O2", "Done": "O3"}},
			items:   []ProjectItem{{ID: "PVTI_1", ContentNodeID: "I_7", Status: status, StatusOptionID: map[string]string{"To Do": "O1", "In Progress": "O2", "Done": "O3"}[status]}},
		}
		app := App{Git: fakeGit{worktrees: []Worktree{{Path: root, Branch: "main", IsRoot: true}}, rootClean: true}, Backlog: bl, GitHub: gh, Logf: func(string, ...any) {}}
		return gh, app
	}
	gh, app := mkApp("In Progress")
	c, err := app.Run(ctx, Config{Root: root, Repo: "owner/repo", ProjectOwner: "owner", ProjectNumber: 8, NoInbox: true})
	if err != nil {
		t.Fatal(err)
	}
	if c.Created != 0 || c.Updated != 0 || c.StatusChanges != 0 || gh.updateProjectStatusCalls != 0 {
		t.Fatalf("expected no-op, counters=%+v projectCalls=%d", c, gh.updateProjectStatusCalls)
	}
	gh, app = mkApp("To Do")
	c, err = app.Run(ctx, Config{Root: root, Repo: "owner/repo", ProjectOwner: "owner", ProjectNumber: 8, NoInbox: true})
	if err != nil {
		t.Fatal(err)
	}
	if c.StatusChanges != 1 || gh.updateProjectStatusCalls != 1 {
		t.Fatalf("expected one project status update, counters=%+v projectCalls=%d", c, gh.updateProjectStatusCalls)
	}
}

func TestInboxImportCreateReplayAndSkip(t *testing.T) {
	ctx := context.Background()
	root := tempRoot(t)
	task := sampleTask()
	inboxIssue := Issue{Number: 44, NodeID: "I_44", HTMLURL: "https://example.test/44", Title: "New idea", Body: "Please do it", State: "open", Labels: []IssueLabel{{Name: "inbox"}}}
	t.Run("create path", func(t *testing.T) {
		bl := newFakeBacklog(root, task)
		bl.createdID = "TASK-99"
		gh := basicGH([]Issue{inboxIssue})
		app := App{Git: fakeGit{worktrees: []Worktree{{Path: root, Branch: "main", IsRoot: true}}, rootClean: true}, Backlog: bl, GitHub: gh, Logf: func(string, ...any) {}}
		c, err := app.Run(ctx, Config{Root: root, Repo: "owner/repo", ProjectOwner: "owner", ProjectNumber: 8})
		if err != nil {
			t.Fatal(err)
		}
		if c.Imported != 1 || len(bl.created) != 1 || len(gh.updatedIssues) != 1 {
			t.Fatalf("expected one imported task and issue update, counters=%+v created=%d updates=%d", c, len(bl.created), len(gh.updatedIssues))
		}
		patch := gh.updatedIssues[0]
		if patch.Title == nil || !strings.HasPrefix(*patch.Title, "task-99:") || patch.Body == nil || !strings.HasPrefix(*patch.Body, "<!-- backlog:task-99 -->") {
			t.Fatalf("inbox issue not marked with created task: %+v", patch)
		}
	})
	t.Run("crash replay guard reuses existing task", func(t *testing.T) {
		replayed := task
		replayed.ID = "TASK-88"
		replayed.Description = "Imported from GitHub issue #44: https://example.test/44"
		bl := newFakeBacklog(root, task, replayed)
		gh := basicGH([]Issue{inboxIssue})
		app := App{Git: fakeGit{worktrees: []Worktree{{Path: root, Branch: "main", IsRoot: true}}, rootClean: true}, Backlog: bl, GitHub: gh, Logf: func(string, ...any) {}}
		c, err := app.Run(ctx, Config{Root: root, Repo: "owner/repo", ProjectOwner: "owner", ProjectNumber: 8})
		if err != nil {
			t.Fatal(err)
		}
		if c.Imported != 0 || len(bl.created) != 0 || len(gh.updatedIssues) != 1 || gh.updatedIssues[0].Title == nil || !strings.HasPrefix(*gh.updatedIssues[0].Title, "task-88:") {
			t.Fatalf("expected replay reuse without task create, counters=%+v created=%d updates=%+v", c, len(bl.created), gh.updatedIssues)
		}
	})
	t.Run("skip when root is not clean main", func(t *testing.T) {
		bl := newFakeBacklog(root, task)
		gh := basicGH([]Issue{inboxIssue})
		app := App{Git: fakeGit{worktrees: []Worktree{{Path: root, Branch: "feature", IsRoot: true}}, rootClean: false, rootReason: "root branch is feature"}, Backlog: bl, GitHub: gh, Logf: func(string, ...any) {}}
		c, err := app.Run(ctx, Config{Root: root, Repo: "owner/repo", ProjectOwner: "owner", ProjectNumber: 8})
		if err != nil {
			t.Fatal(err)
		}
		if c.Imported != 0 || len(bl.created) != 0 || len(gh.updatedIssues) != 0 {
			t.Fatalf("expected skip without writes, counters=%+v created=%d updates=%d", c, len(bl.created), len(gh.updatedIssues))
		}
	})
}

func TestAbortOnEmptyMainTasks(t *testing.T) {
	root := tempRoot(t)
	bl := &fakeBacklog{statuses: []string{"To Do", "In Progress", "Done"}, tasksByDir: map[string][]Task{root: {}}}
	gh := basicGH(nil)
	app := App{Git: fakeGit{worktrees: []Worktree{{Path: root, Branch: "main", IsRoot: true}}}, Backlog: bl, GitHub: gh, Logf: func(string, ...any) {}}
	_, err := app.Run(context.Background(), Config{Root: root, Repo: "owner/repo", ProjectOwner: "owner", ProjectNumber: 8})
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
	return Task{
		ID:                 "TASK-16",
		Title:              "Backlog sync",
		Status:             "In Progress",
		Labels:             []string{"tooling", "ci"},
		Milestone:          sp("m-0"),
		ParentTaskID:       sp("TASK-1"),
		Assignees:          []string{"@pi-worker"},
		CreatedAt:          tp("2026-10-01T02:33:00Z"),
		UpdatedAt:          tp("2026-10-01T02:35:00Z"),
		Description:        "Build a sync.",
		Dependencies:       []string{"TASK-1"},
		Subtasks:           []TaskRef{{ID: "TASK-16.1", Title: "Child"}},
		AcceptanceCriteria: []AcceptanceCriterion{{Index: 1, Text: "First", Checked: true}, {Index: 2, Text: "Second", Checked: false}},
		FinalSummary:       sp("Finished."),
		Branch:             "task-16-backlog-sync",
	}
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

type fakeGit struct {
	worktrees  []Worktree
	rootClean  bool
	rootReason string
}

func (g fakeGit) Worktrees(context.Context, string) ([]Worktree, error) { return g.worktrees, nil }
func (g fakeGit) RootBranchClean(context.Context, string) (bool, string, error) {
	return g.rootClean, g.rootReason, nil
}

type fakeBacklog struct {
	statuses   []string
	tasksByDir map[string][]Task
	createdID  string
	created    []Task
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
	var next *int
	if end < len(tasks) {
		n := end
		next = &n
	}
	return TaskListResponse{Tasks: summaries, Total: len(tasks), NextSkip: next}, nil
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
func (b *fakeBacklog) CreateTask(_ context.Context, _ string, title, description string, labels []string) (string, error) {
	b.created = append(b.created, Task{ID: b.createdID, Title: title, Description: description, Labels: labels, Status: "To Do"})
	return b.createdID, nil
}

type fakeGitHub struct {
	issues                   []Issue
	project                  ProjectInfo
	items                    []ProjectItem
	listIssuesCalls          int
	updatedIssues            []IssuePatch
	createdIssues            []Issue
	addProjectItemCalls      int
	updateProjectStatusCalls int
}

func basicGH(issues []Issue) *fakeGitHub {
	return &fakeGitHub{
		issues:  issues,
		project: ProjectInfo{ID: "P", StatusFieldID: "F", StatusOptionID: map[string]string{"To Do": "O1", "In Progress": "O2", "Done": "O3"}},
		items:   []ProjectItem{},
	}
}

func (g *fakeGitHub) ListIssues(context.Context, string) ([]Issue, error) {
	g.listIssuesCalls++
	return append([]Issue(nil), g.issues...), nil
}
func (g *fakeGitHub) CreateIssue(_ context.Context, _ string, title string, body string, labels []string) (Issue, error) {
	issue := Issue{Number: 100 + len(g.createdIssues), NodeID: "I_NEW", Title: title, Body: body, State: "open"}
	for _, label := range labels {
		issue.Labels = append(issue.Labels, IssueLabel{Name: label})
	}
	g.createdIssues = append(g.createdIssues, issue)
	return issue, nil
}
func (g *fakeGitHub) UpdateIssue(_ context.Context, _ string, number int, patch IssuePatch) (Issue, error) {
	g.updatedIssues = append(g.updatedIssues, patch)
	for _, issue := range g.issues {
		if issue.Number == number {
			return issue, nil
		}
	}
	return Issue{Number: number}, nil
}
func (g *fakeGitHub) EnsureLabel(context.Context, string, string) error { return nil }
func (g *fakeGitHub) ProjectInfo(context.Context, string, int) (ProjectInfo, error) {
	return g.project, nil
}
func (g *fakeGitHub) ListProjectItems(context.Context, string) ([]ProjectItem, error) {
	return append([]ProjectItem(nil), g.items...), nil
}
func (g *fakeGitHub) AddProjectItem(_ context.Context, _ string, contentNodeID string) (ProjectItem, error) {
	g.addProjectItemCalls++
	item := ProjectItem{ID: "PVTI_NEW", ContentNodeID: contentNodeID}
	g.items = append(g.items, item)
	return item, nil
}
func (g *fakeGitHub) UpdateProjectStatus(context.Context, string, string, string, string) error {
	g.updateProjectStatusCalls++
	return nil
}
