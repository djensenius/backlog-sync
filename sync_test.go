package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
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

func TestRenderIssueBodyTruncationBoundaries(t *testing.T) {
	notes := strings.Repeat("notes line with é and 🙂\n", 20)
	doc := issueBodyDocument{
		TaskID:              "task-16",
		Header:              MarkerFor("TASK-16") + "\nmetadata survives\n\n",
		Description:         "description",
		AcceptanceCriteria:  "## Acceptance criteria\nNone\n",
		ImplementationPlan:  sp("plan text"),
		ImplementationNotes: &notes,
		FinalSummary:        sp("summary text"),
	}
	exactBudget := countCharacters(doc.String())
	if got := renderIssueBodyWithinBudget(doc, exactBudget); got != doc.String() || strings.Contains(got, "truncated") {
		t.Fatalf("exact budget should not truncate\ngot=%s", got)
	}
	justOverBudget := exactBudget - 1
	got := renderIssueBodyWithinBudget(doc, justOverBudget)
	if countCharacters(got) > justOverBudget {
		t.Fatalf("just-over render is %d characters, budget %d", countCharacters(got), justOverBudget)
	}
	if !strings.Contains(got, "… truncated (") || !strings.Contains(got, "see `backlog task view task-16 --plain`") {
		t.Fatalf("truncation marker missing from just-over render\n%s", got)
	}
	if !strings.HasPrefix(got, MarkerFor("TASK-16")+"\nmetadata survives") {
		t.Fatalf("marker/metadata prefix changed\n%s", got)
	}

	farOver := sampleTask()
	farOver.ParentTaskID = nil
	farOver.Description = strings.Repeat("description line with é\n", 600)
	farOver.ImplementationPlan = strings.Repeat("plan line\n", 600)
	farOver.ImplementationNotes = strings.Repeat("notes line with 🙂\n", 5000)
	farOver.FinalSummary = sp(strings.Repeat("summary line\n", 600))
	got = RenderIssueBodyWithOptions(farOver, RenderOptions{})
	if chars := countCharacters(got); chars > issueBodyCharacterBudget || chars > githubIssueBodyCharacterLimit {
		t.Fatalf("far-over render is %d characters, budget %d hard limit %d", chars, issueBodyCharacterBudget, githubIssueBodyCharacterLimit)
	}
	if !strings.Contains(got, "## Implementation notes") || !strings.Contains(got, "… truncated (") || !strings.Contains(got, "🙂") {
		t.Fatalf("far-over multi-byte notes were not visibly truncated\n%s", got)
	}
	if !strings.Contains(got, "## Implementation plan\nplan line") || !strings.Contains(got, "## Final summary\nsummary line") {
		t.Fatalf("sections later in the shrink order were truncated before notes required it\n%s", got)
	}
}

func TestRenderIssueBodyMultipleOversizedSectionsStayWithinBudget(t *testing.T) {
	task := sampleTask()
	task.ParentTaskID = nil
	task.Dependencies = nil
	task.Subtasks = nil
	task.Description = strings.Repeat("description line\n", 3900)
	task.ImplementationPlan = ""
	task.ImplementationNotes = strings.Repeat("notes line\n", 6100)
	task.FinalSummary = nil

	body := RenderIssueBodyWithOptions(task, RenderOptions{})
	if chars := countCharacters(body); chars > issueBodyCharacterBudget || chars > githubIssueBodyCharacterLimit {
		t.Fatalf("render is %d characters, budget %d hard limit %d", chars, issueBodyCharacterBudget, githubIssueBodyCharacterLimit)
	}
	notes := markdownSection(t, body, "Implementation notes")
	if strings.Contains(notes, "notes line") || !strings.Contains(notes, "… truncated (") {
		t.Fatalf("oversized notes should be reduced to the truncation note before later sections shrink\n%s", notes)
	}
	description := markdownSection(t, body, "Description")
	if !strings.Contains(description, "description line") || !strings.Contains(description, "… truncated (") {
		t.Fatalf("description should be truncated after notes are minimized\n%s", description)
	}
	if again := RenderIssueBodyWithOptions(task, RenderOptions{}); again != body {
		t.Fatalf("truncated render is not deterministic")
	}
}

func TestRenderIssueBodyShrinkOrderMinimizesNotesBeforeDescription(t *testing.T) {
	task := sampleTask()
	task.ParentTaskID = nil
	task.Dependencies = nil
	task.Subtasks = nil
	task.Description = strings.Repeat("description line\n", 3900)
	task.ImplementationPlan = ""
	task.ImplementationNotes = strings.Repeat("note\n", 1000)
	task.FinalSummary = nil

	body := RenderIssueBodyWithOptions(task, RenderOptions{})
	if chars := countCharacters(body); chars > issueBodyCharacterBudget || chars > githubIssueBodyCharacterLimit {
		t.Fatalf("render is %d characters, budget %d hard limit %d", chars, issueBodyCharacterBudget, githubIssueBodyCharacterLimit)
	}
	notes := markdownSection(t, body, "Implementation notes")
	if strings.Contains(notes, "note\n") || !strings.Contains(notes, "… truncated (") {
		t.Fatalf("notes should be minimized first even when that alone cannot fit the body\n%s", notes)
	}
	description := markdownSection(t, body, "Description")
	if !strings.Contains(description, "description line") || !strings.Contains(description, "… truncated (") {
		t.Fatalf("description should shrink only after notes are minimized\n%s", description)
	}
}

func TestRenderIssueBodyTruncatesOnLineBoundary(t *testing.T) {
	notes := "alpha\n" + strings.Repeat("bravo", 200) + "\ncharlie\n"
	doc := issueBodyDocument{
		TaskID:              "task-77",
		Header:              MarkerFor("TASK-77") + "\nmetadata survives\n\n",
		Description:         "description",
		AcceptanceCriteria:  "## Acceptance criteria\nNone\n",
		ImplementationNotes: &notes,
	}
	candidateDoc := doc
	candidate := truncatedIssueSectionContent(notes, len("alpha"), doc.TaskID)
	candidateDoc.ImplementationNotes = &candidate
	got := renderIssueBodyWithinBudget(doc, countCharacters(candidateDoc.String()))
	notesSection := markdownSection(t, got, "Implementation notes")
	if !strings.HasPrefix(notesSection, "alpha\n… truncated (") || strings.Contains(notesSection, "bravo") {
		t.Fatalf("truncation should keep complete lines only\n%s", notesSection)
	}
}

func TestRenderIssueBodyClosesFenceBeforeTruncationNote(t *testing.T) {
	notes := "intro\n```\ncode line\n" + strings.Repeat("more code\n", 1000)
	doc := issueBodyDocument{
		TaskID:              "task-78",
		Header:              MarkerFor("TASK-78") + "\nmetadata survives\n\n",
		Description:         "description",
		AcceptanceCriteria:  "## Acceptance criteria\nNone\n",
		ImplementationNotes: &notes,
	}
	candidateDoc := doc
	candidate := truncatedIssueSectionContent(notes, len("intro\n```\ncode line"), doc.TaskID)
	candidateDoc.ImplementationNotes = &candidate
	got := renderIssueBodyWithinBudget(doc, countCharacters(candidateDoc.String()))
	notesSection := markdownSection(t, got, "Implementation notes")
	if !strings.Contains(notesSection, "code line\n```\n… truncated (") {
		t.Fatalf("open code fence should be closed before the truncation note\n%s", notesSection)
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

func TestAppCreateFailureIsolatedToOneTask(t *testing.T) {
	ctx := context.Background()
	root := tempRoot(t)
	first := sampleTask()
	first.ID = "TASK-40"
	first.Title = "First succeeds"
	first.ParentTaskID = nil
	second := sampleTask()
	second.ID = "TASK-41"
	second.Title = "Create fails"
	second.ParentTaskID = nil
	third := sampleTask()
	third.ID = "TASK-42"
	third.Title = "Third succeeds"
	third.ParentTaskID = nil
	bl := newFakeBacklog(root, first, second, third)
	gh := basicGH(map[string][]Issue{"owner/repo": {}})
	gh.createErrTitleContains = "task-41:"
	var logs []string
	app := App{Git: fakeGit{worktrees: []Worktree{{Path: root, Branch: "main", IsRoot: true}}, rootClean: true}, Backlog: bl, GitHub: gh, Logf: func(f string, args ...any) { logs = append(logs, fmt.Sprintf(f, args...)) }}
	c, err := app.Run(ctx, testConfig(root))
	if err == nil || !strings.Contains(err.Error(), "1 task sync operation") {
		t.Fatalf("expected non-zero run error for one failed task, counters=%+v err=%v logs=%v", c, err, logs)
	}
	if c.Failed != 1 || c.Created != 2 || len(gh.createdIssues) != 2 {
		t.Fatalf("create failure should not block other tasks, counters=%+v created=%d logs=%v", c, len(gh.createdIssues), logs)
	}
	if !logContains(logs, "error: task-41 create issue failed") || !logContains(logs, "sync complete:") || !logContains(logs, "1 failed") {
		t.Fatalf("failure and summary logs should include task id and failed count, logs=%v", logs)
	}
	if findIssueByTitlePrefix(gh.issues["owner/repo"], "task-40:").Number == 0 || findIssueByTitlePrefix(gh.issues["owner/repo"], "task-42:").Number == 0 {
		t.Fatalf("successful tasks were not created, issues=%+v", gh.issues["owner/repo"])
	}
}

func TestAppOversizedMetadataBodyFailureSkipsCreate(t *testing.T) {
	ctx := context.Background()
	root := tempRoot(t)
	task := sampleTask()
	task.ParentTaskID = nil
	task.Dependencies = nil
	task.Subtasks = nil
	task.Labels = nil
	task.Description = "short"
	task.ImplementationPlan = ""
	task.ImplementationNotes = ""
	task.FinalSummary = nil
	task.AcceptanceCriteria = []AcceptanceCriterion{{Index: 1, Text: strings.Repeat("oversized metadata ", 5000)}}
	bl := newFakeBacklog(root, task)
	gh := basicGH(map[string][]Issue{"owner/repo": {}})
	var logs []string
	app := App{Git: fakeGit{worktrees: []Worktree{{Path: root, Branch: "main", IsRoot: true}}, rootClean: true}, Backlog: bl, GitHub: gh, Logf: func(f string, args ...any) { logs = append(logs, fmt.Sprintf(f, args...)) }}
	c, err := app.Run(ctx, testConfig(root))
	if err == nil || !strings.Contains(err.Error(), "1 task sync operation") {
		t.Fatalf("expected oversized rendered body to fail the task, counters=%+v err=%v logs=%v", c, err, logs)
	}
	if c.Failed != 1 || len(gh.createdIssues) != 0 || gh.ensureLabelCalls != 0 {
		t.Fatalf("oversized body should skip create and pre-create writes, counters=%+v created=%d labels=%d logs=%v", c, len(gh.createdIssues), gh.ensureLabelCalls, logs)
	}
	if !logContains(logs, "render issue body failed") || !logContains(logs, "GitHub hard limit") || !logContains(logs, "1 failed operations") {
		t.Fatalf("oversized body failure should be clear in logs: %v", logs)
	}
}

func TestAppProjectWriteFailureIsolatedToOneTask(t *testing.T) {
	ctx := context.Background()
	root := tempRoot(t)
	cfg := testConfig(root)
	first := sampleTask()
	first.ID = "TASK-60"
	first.Title = "Project add fails"
	first.ParentTaskID = nil
	first.Dependencies = nil
	first.Subtasks = nil
	second := sampleTask()
	second.ID = "TASK-61"
	second.Title = "Create still succeeds"
	second.ParentTaskID = nil
	second.Dependencies = nil
	second.Subtasks = nil
	existing := Issue{Number: 60, DatabaseID: 60, NodeID: "I_60", HTMLURL: "https://github.com/owner/repo/issues/60", Title: IssueTitle(first), Body: RenderIssueBodyWithOptions(first, RenderOptions{}), State: "open", Repo: "owner/repo", Labels: labelsToIssueLabels(DesiredLabels(first, Issue{}, cfg))}
	bl := newFakeBacklog(root, first, second)
	gh := basicGH(map[string][]Issue{"owner/repo": {existing}})
	gh.items = nil
	gh.addProjectItemErr = errors.New("project add failed")
	var logs []string
	app := App{Git: fakeGit{worktrees: []Worktree{{Path: root, Branch: "main", IsRoot: true}}, rootClean: true}, Backlog: bl, GitHub: gh, Logf: func(f string, args ...any) { logs = append(logs, fmt.Sprintf(f, args...)) }}
	c, err := app.Run(ctx, cfg)
	if err == nil || !strings.Contains(err.Error(), "1 task sync operation") {
		t.Fatalf("expected non-zero run error for project write failure, counters=%+v err=%v logs=%v", c, err, logs)
	}
	if c.Failed != 1 || c.Created != 1 || len(gh.createdIssues) != 1 || !logContains(logs, "task-60 add issue to project failed") {
		t.Fatalf("project write failure should not block other tasks, counters=%+v created=%d logs=%v", c, len(gh.createdIssues), logs)
	}
	if findIssueByTitlePrefix(gh.issues["owner/repo"], "task-61:").Number == 0 {
		t.Fatalf("second task was not created after project write failure, issues=%+v", gh.issues["owner/repo"])
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

func TestInboxManualModeReportsNeedsTriageWithoutWrites(t *testing.T) {
	ctx := context.Background()
	root := tempRoot(t)
	task := sampleTask()
	task.Project = nil
	task.ParentTaskID = nil
	task.Dependencies = nil
	task.Subtasks = nil
	cfg := testConfig(root)
	cfg.Inbox.Mode = InboxModeManual
	body := RenderIssueBodyWithOptions(task, RenderOptions{Milestones: map[string]string{"m-0": "v1 release"}, MainBranch: cfg.MainBranch})
	mirrored := Issue{Number: 16, DatabaseID: 16, NodeID: "I_16", HTMLURL: "https://github.com/owner/repo/issues/16", Title: IssueTitle(task), Body: body, State: "open", Repo: "owner/repo", Labels: labelsToIssueLabels(DesiredLabels(task, Issue{}, cfg))}
	inbox := Issue{Number: 90, DatabaseID: 90, NodeID: "I_90", HTMLURL: "https://github.com/owner/repo/issues/90", Title: "Needs owner triage", Body: "body", State: "open", Repo: "owner/repo", Labels: []IssueLabel{{Name: "inbox"}}}
	bl := newFakeBacklog(root, task)
	gh := basicGH(map[string][]Issue{"owner/repo": {mirrored, inbox}})
	gh.failOnWrite = true
	var logs []string
	app := App{Git: fakeGit{worktrees: []Worktree{{Path: root, Branch: task.Branch, IsRoot: true}}, rootClean: false, rootReason: "should not matter"}, Backlog: bl, GitHub: gh, Logf: func(f string, args ...any) { logs = append(logs, fmt.Sprintf(f, args...)) }}
	c, err := app.Run(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if c.InboxTriage != 1 || c.Imported != 0 || len(bl.created) != 0 || bl.pushed || gh.writeCalls() != 0 {
		t.Fatalf("manual mode should report only without writes, counters=%+v creates=%d pushed=%v ghWrites=%d", c, len(bl.created), bl.pushed, gh.writeCalls())
	}
	if !logContains(logs, "inbox issue owner/repo#90 needs triage: Needs owner triage") || !logContains(logs, "1 inbox issues need triage") {
		t.Fatalf("manual mode logs missing needs-triage lines: %v", logs)
	}
}

func TestValidateConfigRejectsInvalidInboxMode(t *testing.T) {
	cfg := testConfig("/repo")
	cfg.Inbox.Mode = "invalid"
	if err := validateConfig(cfg); err == nil || !strings.Contains(err.Error(), "inbox.mode") {
		t.Fatalf("expected invalid inbox.mode error, got %v", err)
	}
}

func TestCollectTasksDiscoversHiddenAndCustomBacklogDirs(t *testing.T) {
	root := t.TempDir()
	hidden := t.TempDir()
	custom := t.TempDir()
	makeBacklogDataDir(t, filepath.Join(root, "custom-backlog"))
	makeBacklogDataDir(t, filepath.Join(hidden, ".backlog"))
	makeBacklogDataDir(t, filepath.Join(custom, "workflow", "data"))
	mainTask := sampleTask()
	mainTask.ID = "TASK-1"
	hiddenTask := sampleTask()
	hiddenTask.ID = "TASK-2"
	customTask := sampleTask()
	customTask.ID = "TASK-3"
	bl := &fakeBacklog{statuses: []string{"To Do", "In Progress", "Done"}, tasksByDir: map[string][]Task{root: {mainTask}, hidden: {hiddenTask}, custom: {customTask}}, createdID: "TASK-99"}
	resolved, err := CollectTasks(context.Background(), testConfig(root), fakeGit{worktrees: []Worktree{{Path: root, Branch: "main", IsRoot: true}, {Path: hidden, Branch: "task-2-fix"}, {Path: custom, Branch: "task-3-fix"}}}, bl, func(string, ...any) {})
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved.Tasks) != 3 || resolved.ByID["task-1"].ID == "" || resolved.ByID["task-2"].ID == "" || resolved.ByID["task-3"].ID == "" {
		t.Fatalf("expected tasks from backlog, .backlog, and custom dirs, got %+v", resolved.ByID)
	}
}

func TestCollectTasksSkipsAmbiguousNonRootBacklogDir(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	makeBacklogDataDir(t, filepath.Join(root, "backlog"))
	makeBacklogDataDir(t, filepath.Join(other, "one"))
	makeBacklogDataDir(t, filepath.Join(other, "two"))
	mainTask := sampleTask()
	mainTask.ID = "TASK-1"
	otherTask := sampleTask()
	otherTask.ID = "TASK-2"
	bl := &fakeBacklog{statuses: []string{"To Do", "In Progress", "Done"}, tasksByDir: map[string][]Task{root: {mainTask}, other: {otherTask}}, createdID: "TASK-99"}
	var logs []string
	resolved, err := CollectTasks(context.Background(), testConfig(root), fakeGit{worktrees: []Worktree{{Path: root, Branch: "main", IsRoot: true}, {Path: other, Branch: "task-2"}}}, bl, func(f string, args ...any) { logs = append(logs, fmt.Sprintf(f, args...)) })
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved.Tasks) != 1 || resolved.ByID["task-2"].ID != "" || !logContains(logs, "ambiguous Backlog directory") {
		t.Fatalf("ambiguous non-root worktree should be skipped with a clear log, tasks=%+v logs=%v", resolved.ByID, logs)
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
	c, err := app.Run(ctx, cfg)
	if err != nil || c.Failed != 0 {
		t.Fatalf("expected unsupported sub-issue rejection to warn without failing, counters=%+v err=%v", c, err)
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

func TestClaimedIssuesPreventDuplicateInboxAndAdoption(t *testing.T) {
	ctx := context.Background()
	root := tempRoot(t)
	task := sampleTask()
	task.ParentTaskID = nil
	task.References = []string{"https://github.com/owner/repo/issues/2"}
	marked := Issue{Number: 1, DatabaseID: 1, NodeID: "I_1", HTMLURL: "https://github.com/owner/repo/issues/1", Title: IssueTitle(task), Body: MarkerFor(task.ID) + "\nold", State: "open", Repo: "owner/repo"}
	extraInbox := Issue{Number: 2, DatabaseID: 2, NodeID: "I_2", HTMLURL: "https://github.com/owner/repo/issues/2", Title: "Extra", Body: "unmarked", State: "open", Repo: "owner/repo", Labels: []IssueLabel{{Name: "inbox"}}}
	bl := newFakeBacklog(root, task)
	gh := basicGH(map[string][]Issue{"owner/repo": {marked, extraInbox}})
	var logs []string
	app := App{Git: fakeGit{worktrees: []Worktree{{Path: root, Branch: "main", IsRoot: true}}, rootClean: true}, Backlog: bl, GitHub: gh, Logf: func(f string, args ...any) { logs = append(logs, fmt.Sprintf(f, args...)) }}
	_, err := app.Run(ctx, testConfig(root))
	if err != nil {
		t.Fatal(err)
	}
	if got := gh.issues["owner/repo"][1]; strings.HasPrefix(got.Body, "<!-- backlog:") || hasLabelFold(got, "inbox") {
		t.Fatalf("extra referenced inbox issue should only have inbox stripped, got %+v", got)
	}
	if !logContains(logs, "refusing to mark inbox issue") {
		t.Fatalf("expected duplicate inbox warning, logs=%v", logs)
	}

	other := sampleTask()
	other.ID = "TASK-17"
	other.ParentTaskID = nil
	other.References = []string{"https://github.com/owner/repo/issues/9"}
	another := other
	another.ID = "TASK-18"
	another.References = []string{"https://github.com/owner/repo/issues/9"}
	gh = basicGH(map[string][]Issue{"owner/repo": {{Number: 9, DatabaseID: 9, NodeID: "I_9", HTMLURL: "https://github.com/owner/repo/issues/9", Title: "Shared", Body: "", State: "open", Repo: "owner/repo"}}})
	bl = newFakeBacklog(root, other, another)
	cfg := testConfig(root)
	cfg.AdoptReferencedIssues = true
	app = App{Git: fakeGit{worktrees: []Worktree{{Path: root, Branch: "main", IsRoot: true}}, rootClean: true}, Backlog: bl, GitHub: gh, Logf: func(string, ...any) {}}
	_, err = app.Run(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(gh.issues["owner/repo"][0].Body, "<!-- backlog:") > 1 || !strings.HasPrefix(gh.issues["owner/repo"][0].Title, "task-17:") {
		t.Fatalf("shared issue should be adopted by first task only, got %+v", gh.issues["owner/repo"][0])
	}
}

func TestMultiReferenceInboxClaimedBeforeAdoption(t *testing.T) {
	ctx := context.Background()
	root := tempRoot(t)
	first := sampleTask()
	first.ID = "TASK-20"
	first.ParentTaskID = nil
	first.References = []string{"https://github.com/owner/repo/issues/20"}
	second := first
	second.ID = "TASK-21"
	second.References = []string{"https://github.com/owner/repo/issues/20"}
	issue := Issue{Number: 20, DatabaseID: 20, NodeID: "I_20", HTMLURL: "https://github.com/owner/repo/issues/20", Title: "Inbox", Body: "body", State: "open", Repo: "owner/repo", Labels: []IssueLabel{{Name: "inbox"}}}
	bl := newFakeBacklog(root, first, second)
	gh := basicGH(map[string][]Issue{"owner/repo": {issue}})
	cfg := testConfig(root)
	cfg.AdoptReferencedIssues = true
	app := App{Git: fakeGit{worktrees: []Worktree{{Path: root, Branch: "main", IsRoot: true}}, rootClean: true}, Backlog: bl, GitHub: gh, Logf: func(string, ...any) {}}
	_, err := app.Run(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(gh.issues["owner/repo"][0].Title, "task-20:") || !strings.HasPrefix(gh.issues["owner/repo"][0].Body, "<!-- backlog:task-20 -->") {
		t.Fatalf("inbox issue should stay claimed by first reference, got %+v", gh.issues["owner/repo"][0])
	}
}

func TestNonTaskPrefixUsedForMarkersCreateIDAndBranch(t *testing.T) {
	if id, ok := ParseMarkerWithPrefix("<!-- backlog:bug-1.2 -->\nbody", "bug"); !ok || id != "bug-1.2" {
		t.Fatalf("non-task marker did not parse: %q %v", id, ok)
	}
	if id, ok := ParseMarkerWithPrefix("<!-- backlog:task-1 -->\nbody", "bug"); ok || id != "" {
		t.Fatalf("task marker should not parse with bug prefix: %q %v", id, ok)
	}
	if !BranchOwnsTaskWithPrefix("bug-1", "BUG-1-fix", "bug") || BranchOwnsTaskWithPrefix("task-1", "task-1-fix", "bug") {
		t.Fatalf("non-task branch rule failed")
	}
	root := tempRoot(t)
	task := sampleTask()
	task.ID = "BUG-1"
	task.ParentTaskID = nil
	bl := newFakeBacklog(root, task)
	bl.taskPrefix = "bug"
	gh := basicGH(map[string][]Issue{"owner/repo": {{Number: 1, DatabaseID: 1, NodeID: "I_BUG", HTMLURL: "u", Title: "Old", Body: "<!-- backlog:bug-1 -->\nold", State: "open", Repo: "owner/repo"}}})
	app := App{Git: fakeGit{worktrees: []Worktree{{Path: root, Branch: "BUG-1-fix", IsRoot: true}}, rootClean: true}, Backlog: bl, GitHub: gh, Logf: func(string, ...any) {}}
	if _, err := app.Run(context.Background(), testConfig(root)); err != nil {
		t.Fatal(err)
	}
	if len(gh.createdIssues) != 0 || !strings.HasPrefix(gh.issues["owner/repo"][0].Title, "bug-1:") {
		t.Fatalf("non-task prefix should update existing bug issue, created=%d issue=%+v", len(gh.createdIssues), gh.issues["owner/repo"][0])
	}
}

func TestTwoRunNoopIncludesBulkCreatesCrossLinksAndDone(t *testing.T) {
	ctx := context.Background()
	root := tempRoot(t)
	parent := sampleTask()
	parent.ID = "TASK-30"
	parent.ParentTaskID = nil
	parent.Subtasks = []TaskRef{{ID: "TASK-30.1", Title: "Child"}}
	child := sampleTask()
	child.ID = "TASK-30.1"
	child.Status = "Done"
	child.ParentTaskID = sp("TASK-30")
	child.Dependencies = []string{"TASK-30"}
	cfg := testConfig(root)
	bl := newFakeBacklog(root, parent, child)
	gh := basicGH(map[string][]Issue{"owner/repo": {}})
	app := App{Git: fakeGit{worktrees: []Worktree{{Path: root, Branch: "main", IsRoot: true}}, rootClean: true}, Backlog: bl, GitHub: gh, Logf: func(string, ...any) {}}
	if _, err := app.Run(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	if len(gh.createdIssues) != 2 {
		t.Fatalf("expected two creates, got %d", len(gh.createdIssues))
	}
	childIssue := findIssueByTitlePrefix(gh.issues["owner/repo"], "task-30.1:")
	if childIssue.State != "closed" {
		t.Fatalf("Done-on-create should close issue in first run, got %q", childIssue.State)
	}
	parentIssue := findIssueByTitlePrefix(gh.issues["owner/repo"], "task-30:")
	if !strings.Contains(parentIssue.Body, "https://github.com/owner/repo/issues/100") {
		t.Fatalf("second-pass body should include child link, body=%s", parentIssue.Body)
	}
	gh.resetWriteCounters()
	bl.created = nil
	bl.pushCount = 0
	if c, err := app.Run(ctx, cfg); err != nil || c.Created != 0 || c.Updated != 0 || c.StatusChanges != 0 || c.ProjectAdded != 0 || c.FieldChanges != 0 || c.SubIssueLinks != 0 {
		t.Fatalf("second run should be no-op counters=%+v err=%v", c, err)
	}
	if writes := gh.writeCalls(); writes != 0 || len(bl.created) != 0 || bl.pushCount != 0 {
		t.Fatalf("second run wrote unexpectedly gh=%d backlogCreates=%d pushes=%d", writes, len(bl.created), bl.pushCount)
	}
}

func TestDryRunZeroWritesAndAdoptedIssueFieldReporting(t *testing.T) {
	ctx := context.Background()
	root := tempRoot(t)
	task := sampleTask()
	task.ParentTaskID = nil
	task.References = []string{"https://github.com/owner/repo/issues/7"}
	cfg := testConfig(root)
	cfg.DryRun = true
	cfg.AdoptReferencedIssues = true
	cfg.Fields = FieldConfig{Priority: "Priority", Milestone: "Backlog milestone", Area: "Area", TaskID: "Task ID", Branch: "Branch"}
	bl := newFakeBacklog(root, task)
	gh := basicGH(map[string][]Issue{"owner/repo": {{Number: 7, DatabaseID: 7, NodeID: "I_7", HTMLURL: "https://github.com/owner/repo/issues/7", Title: "Old", Body: "old", State: "open", Repo: "owner/repo"}}})
	gh.items = nil
	gh.failOnWrite = true
	gh.project.Fields["Priority"] = ProjectField{ID: "PF", Options: map[string]string{"High": "PH"}}
	gh.project.Fields["Backlog milestone"] = ProjectField{ID: "MF", Options: map[string]string{"v1 release": "M0"}}
	gh.project.Fields["Area"] = ProjectField{ID: "AF", Options: map[string]string{"apple": "AA"}}
	gh.project.Fields["Task ID"] = ProjectField{ID: "TF"}
	gh.project.Fields["Branch"] = ProjectField{ID: "BF"}
	var logs []string
	app := App{Git: fakeGit{worktrees: []Worktree{{Path: root, Branch: "main", IsRoot: true}}, rootClean: true}, Backlog: bl, GitHub: gh, Logf: func(f string, args ...any) { logs = append(logs, fmt.Sprintf(f, args...)) }}
	if _, err := app.Run(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	if writes := gh.writeCalls(); writes != 0 || len(bl.created) != 0 || bl.pushCount != 0 {
		t.Fatalf("dry run wrote unexpectedly gh=%d backlogCreates=%d pushes=%d", writes, len(bl.created), bl.pushCount)
	}
	if !logContains(logs, "would adopt referenced issue") || !logContains(logs, "would add issue owner/repo#7 to project") || !logContains(logs, "would set project field \"Task ID\"") {
		t.Fatalf("dry-run adopted issue did not report project fields, logs=%v", logs)
	}
}

func TestSubIssueParentDiffAndExpectedRejectionWarnsOnce(t *testing.T) {
	ctx := context.Background()
	root := tempRoot(t)
	cfg := testConfig(root)
	parent := Issue{Number: 1, DatabaseID: 1, NodeID: "P", Repo: "owner/repo"}
	child := Issue{Number: 2, DatabaseID: 2, NodeID: "C", Repo: "owner/repo"}
	resolved := ResolvedTasks{Tasks: []Task{{ID: "TASK-1"}, {ID: "TASK-1.1", ParentTaskID: sp("TASK-1")}}}
	issues := map[string]Issue{"task-1": parent, "task-1.1": child}
	gh := basicGH(map[string][]Issue{"owner/repo": {parent, child}})
	gh.parents["C"] = IssueParentInfo{ID: "P", Number: 1, Repo: "owner/repo"}
	app := App{GitHub: gh, Logf: func(string, ...any) {}}
	links, failed := app.syncSubIssues(ctx, cfg, resolved, issues)
	if links != 0 || failed != 0 || gh.addSubIssueCalls != 0 || gh.removeSubIssueCalls != 0 {
		t.Fatalf("same parent should be no-op links=%d failed=%d add=%d remove=%d", links, failed, gh.addSubIssueCalls, gh.removeSubIssueCalls)
	}
	gh.parents["C"] = IssueParentInfo{ID: "OLD", Number: 9, Repo: "owner/repo"}
	links, failed = app.syncSubIssues(ctx, cfg, resolved, issues)
	if links != 1 || failed != 0 || gh.removeSubIssueCalls != 1 || gh.addSubIssueCalls != 1 {
		t.Fatalf("different parent should remove then add links=%d failed=%d add=%d remove=%d", links, failed, gh.addSubIssueCalls, gh.removeSubIssueCalls)
	}
	gh = basicGH(map[string][]Issue{"owner/repo": {parent, child}})
	gh.parents["C"] = IssueParentInfo{ID: "FOREIGN", Number: 99, Repo: "evil/repo"}
	app = App{GitHub: gh, Logf: func(string, ...any) {}}
	links, failed = app.syncSubIssues(ctx, cfg, resolved, issues)
	if links != 0 || failed != 0 || gh.writeCalls() != 0 {
		t.Fatalf("foreign parent should skip all writes links=%d failed=%d writes=%d", links, failed, gh.writeCalls())
	}

	child2 := Issue{Number: 3, DatabaseID: 3, NodeID: "C2", Repo: "owner/repo"}
	resolved.Tasks = append(resolved.Tasks, Task{ID: "TASK-1.2", ParentTaskID: sp("TASK-1")})
	issues["task-1.2"] = child2
	gh = basicGH(map[string][]Issue{"owner/repo": {parent, child, child2}})
	gh.subIssueErr = errors.New("unsupported")
	var logs []string
	app = App{GitHub: gh, Logf: func(f string, args ...any) { logs = append(logs, fmt.Sprintf(f, args...)) }}
	links, failed = app.syncSubIssues(ctx, cfg, resolved, issues)
	if links != 0 || failed != 0 || strings.Count(strings.Join(logs, "\n"), "sub-issue links are not supported") != 1 || gh.addSubIssueCalls != 2 {
		t.Fatalf("expected unsupported sub-issue rejection to warn once without failing, links=%d failed=%d calls=%d logs=%v", links, failed, gh.addSubIssueCalls, logs)
	}

	gh = basicGH(map[string][]Issue{"owner/repo": {parent, child, child2}})
	gh.subIssueErr = errors.New("api exploded")
	logs = nil
	app = App{GitHub: gh, Logf: func(f string, args ...any) { logs = append(logs, fmt.Sprintf(f, args...)) }}
	links, failed = app.syncSubIssues(ctx, cfg, resolved, issues)
	if links != 0 || failed != 2 || strings.Count(strings.Join(logs, "\n"), "add sub-issue") != 2 || !logContains(logs, "task-1.1") || !logContains(logs, "task-1.2") || gh.addSubIssueCalls != 2 {
		t.Fatalf("expected unexpected sub-issue API errors to fail per task, links=%d failed=%d calls=%d logs=%v", links, failed, gh.addSubIssueCalls, logs)
	}
}

func TestDuplicateMarkersAbortOnCLIFailureAndPaginationMissingTotal(t *testing.T) {
	logs := []string{}
	issues := []Issue{{Number: 2, Repo: "owner/repo", Body: MarkerFor("TASK-1") + "\n"}, {Number: 1, Repo: "owner/repo", Body: MarkerFor("TASK-1") + "\n"}}
	indexed, _ := IndexIssues(issues, "inbox", "task", func(f string, args ...any) { logs = append(logs, fmt.Sprintf(f, args...)) })
	if indexed["task-1"].Number != 1 || !logContains(logs, "duplicate marker") {
		t.Fatalf("duplicate marker handling failed indexed=%+v logs=%v", indexed, logs)
	}
	root := tempRoot(t)
	bl := newFakeBacklog(root, sampleTask())
	bl.taskPrefixErr = errors.New("config failed")
	gh := basicGH(map[string][]Issue{"owner/repo": {}})
	app := App{Git: fakeGit{worktrees: []Worktree{{Path: root, Branch: "main", IsRoot: true}}}, Backlog: bl, GitHub: gh, Logf: func(string, ...any) {}}
	if _, err := app.Run(context.Background(), testConfig(root)); err == nil || !strings.Contains(err.Error(), "task prefix") {
		t.Fatalf("expected hard prefix error, got %v", err)
	}
	fullPage := make([]Task, 100)
	for i := range fullPage {
		fullPage[i] = sampleTask()
		fullPage[i].ID = fmt.Sprintf("TASK-%d", i+100)
	}
	bl = newFakeBacklog(root, fullPage...)
	bl.missingTotal = true
	if _, err := listAllTasks(context.Background(), bl, root); err == nil || !strings.Contains(err.Error(), "missing total") {
		t.Fatalf("expected missing total error, got %v", err)
	}
}

func TestSingleSelectClearVerboseDedupHelpAndDeferredPush(t *testing.T) {
	ctx := context.Background()
	root := tempRoot(t)
	cfg := testConfig(root)
	cfg.Fields = FieldConfig{Priority: "Priority"}
	item := ProjectItem{ID: "ITEM", FieldValues: map[string]ProjectFieldValue{"Priority": {Name: "High", OptionID: "PH"}}}
	project := ProjectInfo{ID: "P", StatusFieldID: "SF", StatusOptionID: map[string]string{"To Do": "TODO"}, Fields: map[string]ProjectField{"Priority": {ID: "PF", Options: map[string]string{"High": "PH"}}}}
	task := Task{ID: "TASK-1", Status: "To Do"}
	gh := basicGH(map[string][]Issue{"owner/repo": {}})
	var logs []string
	app := App{GitHub: gh, Logf: func(f string, args ...any) { logs = append(logs, fmt.Sprintf(f, args...)) }}
	changes, err := app.syncProjectFields(ctx, cfg, project, item, task, nil, Issue{Repo: "owner/repo", Number: 1})
	if err != nil || changes.fields != 1 || gh.fieldUpdates != 1 || logContains(logs, "has no option") {
		t.Fatalf("empty single-select should clear without warning changes=%+v err=%v logs=%v", changes, err, logs)
	}
	cfg.Verbose = true
	app.verboseOnce(cfg, "k", "verbose: once")
	app.verboseOnce(cfg, "k", "verbose: once")
	if strings.Count(strings.Join(logs, "\n"), "verbose: once") != 1 {
		t.Fatalf("verboseOnce did not dedupe logs=%v", logs)
	}
	if _, err := parseFlags([]string{"-h"}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("-h should return flag.ErrHelp, got %v", err)
	}

	inbox := Issue{Number: 55, DatabaseID: 55, NodeID: "I_55", HTMLURL: "https://github.com/owner/repo/issues/55", Title: "Import", Body: "body", State: "open", Repo: "owner/repo", Labels: []IssueLabel{{Name: "inbox"}}}
	existing := sampleTask()
	existing.ParentTaskID = nil
	bl := newFakeBacklog(root, existing)
	bl.createdID = "TASK-55"
	gh = basicGH(map[string][]Issue{"owner/repo": {inbox}})
	gh.updateErr = errors.New("update failed")
	app = App{Git: fakeGit{worktrees: []Worktree{{Path: root, Branch: "main", IsRoot: true}}, rootClean: true}, Backlog: bl, GitHub: gh, Logf: func(string, ...any) {}}
	cfg = testConfig(root)
	cfg.Inbox.Push = true
	if _, err := app.Run(ctx, cfg); err == nil || !strings.Contains(err.Error(), "update failed") {
		t.Fatalf("expected inbox update failure, got %v", err)
	}
	if len(bl.created) != 1 || bl.pushCount != 1 {
		t.Fatalf("import commit should be pushed in deferred path, creates=%d pushes=%d", len(bl.created), bl.pushCount)
	}
}

func TestGitStatusUsesNoOptionalLocks(t *testing.T) {
	r := &scriptRunner{outputs: map[string][]byte{
		"git -C /repo branch --show-current":                     []byte("main\n"),
		"git -C /repo rev-parse --git-path MERGE_HEAD":           []byte("/tmp/missing-merge\n"),
		"git -C /repo rev-parse --git-path rebase-merge":         []byte("/tmp/missing-rebase-merge\n"),
		"git -C /repo rev-parse --git-path rebase-apply":         []byte("/tmp/missing-rebase-apply\n"),
		"git -C /repo rev-parse --git-path CHERRY_PICK_HEAD":     []byte("/tmp/missing-cherry\n"),
		"git --no-optional-locks -C /repo status --porcelain=v1": []byte(""),
	}}
	clean, reason, err := (ExecGit{Runner: r}).RootBranchClean(context.Background(), "/repo", "main")
	if err != nil || !clean || reason != "" {
		t.Fatalf("RootBranchClean=%v %q err=%v", clean, reason, err)
	}
}

func TestExecParsersAndJSONShapes(t *testing.T) {
	wt := ParseWorktrees("worktree /repo\nbranch refs/heads/main\n\nworktree /repo-task\nbranch refs/heads/task-1-fix\n\nworktree /bare\nbare\n", "/repo")
	if len(wt) != 3 || !wt[0].IsRoot || wt[1].Branch != "task-1-fix" || !wt[2].Bare {
		t.Fatalf("bad worktree parse: %+v", wt)
	}
	r := &scriptRunner{outputs: map[string][]byte{
		"backlog config get statuses":                  []byte("Todo, Doing, Done\n"),
		"backlog task create --plain -d desc -- title": []byte("Created BUG-12.3: title\n"),
	}}
	bl := ExecBacklog{Runner: r}
	statuses, err := bl.Statuses(context.Background(), "/repo")
	if err != nil || strings.Join(statuses, ",") != "Todo,Doing,Done" {
		t.Fatalf("statuses parse=%v err=%v", statuses, err)
	}
	id, err := bl.CreateTask(context.Background(), "/repo", CreateTaskInput{Title: "title", Description: "desc", TaskPrefix: "bug"})
	if err != nil || id != "BUG-12.3" {
		t.Fatalf("CreateTask id=%q err=%v", id, err)
	}
	ghRunner := &scriptRunner{outputs: map[string][]byte{
		"gh api --paginate --slurp repos/owner/repo/issues?state=all&per_page=100": []byte(`[[{"number":1,"id":10,"node_id":"I_1","html_url":"u","title":"T","body":"B","state":"open","labels":[{"name":"bug"}]},{"number":2,"pull_request":{}}]]`),
		"gh api graphql --input -": []byte(`{"data":{"user":{"projectV2":{"id":"P","fields":{"nodes":[{"id":"F","name":"Status","options":[{"id":"O1","name":"To Do"}]},{"id":"T","name":"Task ID","dataType":"TEXT"}]}}},"node":{"items":{"nodes":[{"id":"ITEM","content":{"id":"I_1"},"fieldValueByName":{"name":"To Do","optionId":"O1"},"fieldValues":{"nodes":[{"field":{"name":"Task ID"},"text":"task-1"}]} }],"pageInfo":{"hasNextPage":false}}}}}`),
	}}
	gh := ExecGitHub{Runner: ghRunner, AllowedRepos: []string{"owner/repo"}}
	list, err := gh.ListIssues(context.Background(), "owner/repo")
	if err != nil || len(list) != 1 || list[0].Labels[0].Name != "bug" {
		t.Fatalf("REST issue parse=%+v err=%v", list, err)
	}
	project, err := gh.ProjectInfo(context.Background(), "user", "owner", 8)
	if err != nil || project.ID != "P" || project.StatusOptionID["To Do"] != "O1" || project.Fields["Task ID"].ID != "T" {
		t.Fatalf("ProjectInfo parse=%+v err=%v", project, err)
	}
	items, err := gh.ListProjectItems(context.Background(), "P")
	if err != nil || len(items) != 1 || items[0].FieldValues["Task ID"].Text != "task-1" {
		t.Fatalf("ListProjectItems parse=%+v err=%v", items, err)
	}
	ghRunner.outputs["gh api graphql --input -"] = []byte(`{"data":{"node":{"parent":{"id":"PARENT","number":4,"repository":{"nameWithOwner":"owner/repo"}}}}}`)
	parent, err := gh.IssueParent(context.Background(), "I_1")
	if err != nil || parent.Repo != "owner/repo" || parent.Number != 4 || parent.ID != "PARENT" {
		t.Fatalf("IssueParent parse=%+v err=%v", parent, err)
	}

}

func TestTaskViewJSONFixtureFieldNames(t *testing.T) {
	for _, name := range []string{"testdata/task-view-task-16.json", "testdata/task-view-with-reference.json"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		var resp TaskViewResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			t.Fatalf("%s unmarshal: %v", name, err)
		}
		if resp.Task.ID == "" || resp.Task.References == nil || resp.Task.ImplementationPlan == "" || resp.Task.ImplementationNotes == "" {
			t.Fatalf("%s missing expected field names: %+v", name, resp.Task)
		}
		if name == "testdata/task-view-with-reference.json" && (len(resp.Task.References) == 0 || resp.Task.Project == nil || resp.Task.Priority == nil) {
			t.Fatalf("synthetic reference fixture missing references/project/priority: %+v", resp.Task)
		}
	}
}

func tempRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	makeBacklogDataDir(t, filepath.Join(root, "backlog"))
	return root
}

func makeBacklogDataDir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(path, "tasks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "config.yml"), []byte("statuses: [\"To Do\", \"In Progress\", \"Done\"]\ntask_prefix: \"task\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sampleTask() Task {
	return Task{ID: "TASK-16", Title: "Backlog sync", Status: "In Progress", Project: sp("apple"), Priority: sp("High"), Labels: []string{"tooling", "ci"}, Milestone: sp("m-0"), ParentTaskID: sp("TASK-1"), Assignees: []string{"@pi-worker"}, CreatedAt: tp("2026-10-01T02:33:00Z"), UpdatedAt: tp("2026-10-01T02:35:00Z"), Description: "Build a sync.", Dependencies: []string{"TASK-1"}, Subtasks: []TaskRef{{ID: "TASK-16.1", Title: "Child"}}, AcceptanceCriteria: []AcceptanceCriterion{{Index: 1, Text: "First", Checked: true}, {Index: 2, Text: "Second", Checked: false}}, ImplementationPlan: "Plan it.", ImplementationNotes: "Built it.", FinalSummary: sp("Finished."), Branch: "task-16-backlog-sync"}
}

func markdownSection(t *testing.T, body, heading string) string {
	t.Helper()
	startMarker := "## " + heading + "\n"
	start := strings.Index(body, startMarker)
	if start < 0 {
		t.Fatalf("section %q not found in body\n%s", heading, body)
	}
	section := body[start+len(startMarker):]
	if end := strings.Index(section, "\n## "); end >= 0 {
		section = section[:end]
	}
	return strings.TrimRight(section, "\n")
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func logContains(logs []string, needle string) bool {
	return strings.Contains(strings.Join(logs, "\n"), needle)
}

func findIssueByTitlePrefix(issues []Issue, prefix string) Issue {
	for _, issue := range issues {
		if strings.HasPrefix(issue.Title, prefix) {
			return issue
		}
	}
	return Issue{}
}

type scriptRunner struct{ outputs map[string][]byte }

func (r *scriptRunner) Run(_ context.Context, _ string, name string, args []string, _ []byte) ([]byte, error) {
	key := name
	if len(args) > 0 {
		key += " " + strings.Join(args, " ")
	}
	if out, ok := r.outputs[key]; ok {
		return out, nil
	}
	return nil, fmt.Errorf("unexpected command %s", key)
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
	statuses            []string
	tasksByDir          map[string][]Task
	createdID           string
	created             []Task
	createDirs          []string
	pushed              bool
	pushCount           int
	badTotal            bool
	missingTotal        bool
	taskPrefix          string
	taskPrefixErr       error
	statusErr           error
	createErr           error
	viewErr             error
	tasksForUnknownDirs []Task
}

func newFakeBacklog(root string, tasks ...Task) *fakeBacklog {
	return &fakeBacklog{statuses: []string{"To Do", "In Progress", "Done"}, tasksByDir: map[string][]Task{root: tasks}, createdID: "TASK-99"}
}
func (b *fakeBacklog) ListTasks(_ context.Context, dir string, maxCount, skip int) (TaskListResponse, error) {
	tasks, ok := b.tasksByDir[dir]
	if !ok {
		tasks = b.tasksForUnknownDirs
	}
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
	var totalPtr *int
	if !b.missingTotal {
		totalPtr = &total
	}
	return TaskListResponse{Tasks: summaries, Total: totalPtr, NextSkip: next}, nil
}
func (b *fakeBacklog) ViewTask(_ context.Context, dir, id string) (TaskViewResponse, error) {
	if b.viewErr != nil {
		return TaskViewResponse{}, b.viewErr
	}
	tasks, ok := b.tasksByDir[dir]
	if !ok {
		tasks = b.tasksForUnknownDirs
	}
	for _, task := range tasks {
		if CanonicalTaskID(task.ID) == CanonicalTaskID(id) {
			return TaskViewResponse{Task: task}, nil
		}
	}
	return TaskViewResponse{}, nil
}
func (b *fakeBacklog) Statuses(context.Context, string) ([]string, error) {
	if b.statusErr != nil {
		return nil, b.statusErr
	}
	return b.statuses, nil
}
func (b *fakeBacklog) TaskPrefix(context.Context, string) (string, error) {
	if b.taskPrefixErr != nil {
		return "", b.taskPrefixErr
	}
	if b.taskPrefix != "" {
		return b.taskPrefix, nil
	}
	return "task", nil
}
func (b *fakeBacklog) Milestones(context.Context, string) (map[string]string, error) {
	return map[string]string{"m-0": "v1 release"}, nil
}
func (b *fakeBacklog) CreateTask(_ context.Context, dir string, in CreateTaskInput) (string, error) {
	if b.createErr != nil {
		return "", b.createErr
	}
	p := in.Project
	task := Task{ID: b.createdID, Title: in.Title, Description: in.Description, Labels: in.Labels, Status: "To Do", Project: &p, References: in.References}
	b.created = append(b.created, task)
	b.createDirs = append(b.createDirs, dir)
	b.tasksByDir[dir] = append(b.tasksByDir[dir], task)
	return b.createdID, nil
}
func (b *fakeBacklog) Push(context.Context, string, string) error {
	b.pushed = true
	b.pushCount++
	return nil
}
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
	ensureLabelCalls         int
	addProjectItemCalls      int
	updateProjectStatusCalls int
	fieldUpdates             int
	parents                  map[string]IssueParentInfo
	addSubIssueCalls         int
	removeSubIssueCalls      int
	subIssueErr              error
	removeSubIssueErr        error
	updateErr                error
	addProjectItemErr        error
	createErrTitleContains   string
	failOnWrite              bool
}

func basicGH(issues map[string][]Issue) *fakeGitHub {
	gh := &fakeGitHub{issues: issues, project: ProjectInfo{ID: "P", StatusFieldID: "F", StatusOptionID: map[string]string{"To Do": "O1", "In Progress": "O2", "Done": "O3"}, Fields: map[string]ProjectField{"Status": {ID: "F", Options: map[string]string{"To Do": "O1", "In Progress": "O2", "Done": "O3"}}}}, parents: map[string]IssueParentInfo{}}
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
	if g.failOnWrite {
		return Issue{}, errors.New("unexpected write")
	}
	if g.createErrTitleContains != "" && strings.Contains(title, g.createErrTitleContains) {
		return Issue{}, errors.New("create failed")
	}
	issue := Issue{Number: 100 + len(g.createdIssues), DatabaseID: int64(100 + len(g.createdIssues)), NodeID: "I_NEW_" + title, HTMLURL: "https://github.com/" + repo + "/issues/100", Title: title, Body: body, State: "open", Repo: repo, Labels: labelsToIssueLabels(labels)}
	g.createdIssues = append(g.createdIssues, issue)
	g.issues[repo] = append(g.issues[repo], issue)
	g.items = append(g.items, ProjectItem{ID: "PVTI_" + issue.NodeID, ContentNodeID: issue.NodeID, Status: "", FieldValues: map[string]ProjectFieldValue{}})
	return issue, nil
}
func (g *fakeGitHub) UpdateIssue(_ context.Context, repo string, number int, patch IssuePatch) (Issue, error) {
	if g.failOnWrite {
		return Issue{}, errors.New("unexpected write")
	}
	if g.updateErr != nil {
		return Issue{}, g.updateErr
	}
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
func (g *fakeGitHub) EnsureLabel(context.Context, string, string) error {
	if g.failOnWrite {
		return errors.New("unexpected write")
	}
	g.ensureLabelCalls++
	return nil
}
func (g *fakeGitHub) ProjectInfo(context.Context, string, string, int) (ProjectInfo, error) {
	return g.project, nil
}
func (g *fakeGitHub) ListProjectItems(context.Context, string) ([]ProjectItem, error) {
	return append([]ProjectItem(nil), g.items...), nil
}
func (g *fakeGitHub) AddProjectItem(_ context.Context, _ string, contentNodeID string) (ProjectItem, error) {
	if g.failOnWrite {
		return ProjectItem{}, errors.New("unexpected write")
	}
	g.addProjectItemCalls++
	if g.addProjectItemErr != nil {
		return ProjectItem{}, g.addProjectItemErr
	}
	item := ProjectItem{ID: "PVTI_NEW", ContentNodeID: contentNodeID, FieldValues: map[string]ProjectFieldValue{}}
	g.items = append(g.items, item)
	return item, nil
}
func (g *fakeGitHub) UpdateProjectStatus(_ context.Context, _ string, itemID string, _ string, optionID string) error {
	if g.failOnWrite {
		return errors.New("unexpected write")
	}
	g.updateProjectStatusCalls++
	for i := range g.items {
		if g.items[i].ID == itemID {
			g.items[i].StatusOptionID = optionID
			for name, id := range g.project.StatusOptionID {
				if id == optionID {
					g.items[i].Status = name
				}
			}
		}
	}
	return nil
}
func (g *fakeGitHub) UpdateProjectSingleSelect(context.Context, string, string, string, string) error {
	if g.failOnWrite {
		return errors.New("unexpected write")
	}
	g.fieldUpdates++
	return nil
}
func (g *fakeGitHub) UpdateProjectText(context.Context, string, string, string, string) error {
	if g.failOnWrite {
		return errors.New("unexpected write")
	}
	g.fieldUpdates++
	return nil
}
func (g *fakeGitHub) ClearProjectField(context.Context, string, string, string) error {
	if g.failOnWrite {
		return errors.New("unexpected write")
	}
	g.fieldUpdates++
	return nil
}
func (g *fakeGitHub) IssueParent(_ context.Context, issueNodeID string) (IssueParentInfo, error) {
	return g.parents[issueNodeID], nil
}
func (g *fakeGitHub) AddSubIssue(_ context.Context, parentRepo string, parentNumber int, childDatabaseID int64) error {
	if g.failOnWrite {
		return errors.New("unexpected write")
	}
	g.addSubIssueCalls++
	if g.subIssueErr != nil {
		return g.subIssueErr
	}
	var parent Issue
	for _, issue := range g.issues[parentRepo] {
		if issue.Number == parentNumber {
			parent = issue
		}
	}
	for _, repoIssues := range g.issues {
		for _, issue := range repoIssues {
			if issue.DatabaseID == childDatabaseID {
				g.parents[issue.NodeID] = IssueParentInfo{ID: parent.NodeID, Number: parent.Number, Repo: parent.Repo}
			}
		}
	}
	return nil
}
func (g *fakeGitHub) RemoveSubIssue(_ context.Context, _ string, childNodeID string) error {
	if g.failOnWrite {
		return errors.New("unexpected write")
	}
	g.removeSubIssueCalls++
	if g.removeSubIssueErr != nil {
		return g.removeSubIssueErr
	}
	delete(g.parents, childNodeID)
	return nil
}
func (g *fakeGitHub) resetWriteCounters() {
	g.updatedIssues = nil
	g.createdIssues = nil
	g.ensureLabelCalls = 0
	g.addProjectItemCalls = 0
	g.updateProjectStatusCalls = 0
	g.fieldUpdates = 0
	g.addSubIssueCalls = 0
	g.removeSubIssueCalls = 0
}

func (g *fakeGitHub) writeCalls() int {
	return len(g.updatedIssues) + len(g.createdIssues) + g.ensureLabelCalls + g.addProjectItemCalls + g.updateProjectStatusCalls + g.fieldUpdates + g.addSubIssueCalls + g.removeSubIssueCalls
}
