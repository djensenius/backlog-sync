package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Use the real prefix adapter with synthetic stdout, not a live Backlog command.
type prefixOutputBacklog struct {
	*fakeBacklog
	output string
}

func (b prefixOutputBacklog) TaskPrefix(ctx context.Context, dir string) (string, error) {
	return (ExecBacklog{Runner: &scriptRunner{outputs: map[string][]byte{
		"backlog config get taskPrefix": []byte(b.output),
	}}}).TaskPrefix(ctx, dir)
}

func TestAppPrefixMismatchAbortsBeforeWrites(t *testing.T) {
	for _, output := range []string{"TASK1\n", "Unknown config key: taskPrefix\n"} {
		t.Run(output, func(t *testing.T) {
			root := tempRoot(t)
			task := Task{ID: "TASK-1", Title: "Synthetic task", Status: "To Do"}
			bl := prefixOutputBacklog{fakeBacklog: newFakeBacklog(root, task), output: output}
			gh := basicGH(map[string][]Issue{"owner/repo": {{
				Number: 1, DatabaseID: 1, NodeID: "I_1", Repo: "owner/repo",
				Title: IssueTitle(task), Body: MarkerFor(task.ID) + "\nold", State: "open",
			}}})
			app := App{Git: fakeGit{worktrees: []Worktree{{Path: root, Branch: "main", IsRoot: true}}}, Backlog: bl, GitHub: gh}
			for run := 1; run <= 2; run++ {
				c, err := app.Run(context.Background(), testConfig(root))
				t.Logf("run %d: created=%d issues=%d err=%v", run, c.Created, len(gh.issues["owner/repo"]), err)
				if err == nil || !strings.Contains(err.Error(), "task identity") {
					t.Errorf("run %d: expected prefix/task identity error before any writes", run)
				}
			}
			if gh.writeCalls() != 0 || len(bl.created) != 0 || bl.pushCount != 0 {
				t.Fatalf("identity mismatch wrote: github=%d backlog=%d pushes=%d issues=%d",
					gh.writeCalls(), len(bl.created), bl.pushCount, len(gh.issues["owner/repo"]))
			}
		})
	}
}

type taskViewOutputBacklog struct {
	*fakeBacklog
	id, output string
}

func (b taskViewOutputBacklog) ViewTask(ctx context.Context, dir, id string) (TaskViewResponse, error) {
	if id != b.id || b.output == "" {
		return b.fakeBacklog.ViewTask(ctx, dir, id)
	}
	return (ExecBacklog{Runner: &scriptRunner{outputs: map[string][]byte{
		"backlog task view " + id + " --json": []byte(b.output),
	}}}).ViewTask(ctx, dir, id)
}

func TestAppTaskIdentityValidationBeforeWrites(t *testing.T) {
	for _, tc := range []struct {
		name, summaryID, viewOutput string
	}{
		{"empty summary", "", ""},
		{"wrong prefix", "BUG-2", ""},
		{"ID substring", "not-task-2", ""},
		{"ID suffix", "TASK-2-extra", ""},
		{"ID trailing dot", "TASK-2.", ""},
		{"embedded marker line", "TASK-2 -->\nignored", ""},
		{"null view", "TASK-2", `null`},
		{"missing task", "TASK-2", `{}`},
		{"null task", "TASK-2", `{"task":null}`},
		{"missing view ID", "TASK-2", `{"task":{"title":"Synthetic task","status":"To Do"}}`},
		{"different view ID", "TASK-2", `{"task":{"id":"TASK-3","title":"Synthetic task","status":"To Do"}}`},
		{"malformed view ID", "TASK-2", `{"task":{"id":"TASK-2-extra","title":"Synthetic task","status":"To Do"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := tempRoot(t)
			bl := taskViewOutputBacklog{
				fakeBacklog: newFakeBacklog(root,
					Task{ID: "TASK-1", Title: "Healthy first task", Status: "To Do"},
					Task{ID: tc.summaryID, Title: "Invalid later task", Status: "To Do"}),
				id: tc.summaryID, output: tc.viewOutput,
			}
			// An inbox issue also must not be imported before the full task snapshot is valid.
			gh := basicGH(map[string][]Issue{"owner/repo": {{
				Number: 9, DatabaseID: 9, NodeID: "I_9", Repo: "owner/repo",
				HTMLURL: "https://github.com/owner/repo/issues/9", Title: "Synthetic inbox",
				State: "open", Labels: []IssueLabel{{Name: "inbox"}},
			}}})
			app := App{Git: fakeGit{worktrees: []Worktree{{Path: root, Branch: "main", IsRoot: true}}, rootClean: true}, Backlog: bl, GitHub: gh}
			c, err := app.Run(context.Background(), testConfig(root))
			if err == nil || !strings.Contains(err.Error(), "task identity") {
				t.Errorf("expected task identity error, counters=%+v err=%v", c, err)
			}
			if gh.writeCalls() != 0 || len(bl.created) != 0 || bl.pushCount != 0 {
				t.Fatalf("invalid snapshot wrote: github=%d backlog=%d pushes=%d", gh.writeCalls(), len(bl.created), bl.pushCount)
			}
		})
	}
}

func TestAppRejectsOtherWorktreePrefixBeforeWrites(t *testing.T) {
	root, other := tempRoot(t), tempRoot(t)
	bl := newFakeBacklog(root, Task{ID: "TASK-1", Title: "Root task", Status: "To Do"})
	bl.tasksByDir[other] = []Task{{ID: "BUG-2", Title: "Other prefix", Status: "To Do"}}
	gh := basicGH(map[string][]Issue{"owner/repo": {}})
	app := App{
		Git: fakeGit{worktrees: []Worktree{
			{Path: root, Branch: "main", IsRoot: true}, {Path: other, Branch: "bug-2-fix"},
		}},
		Backlog: bl, GitHub: gh,
	}
	if _, err := app.Run(context.Background(), testConfig(root)); err == nil || !strings.Contains(err.Error(), "task identity") {
		t.Fatalf("expected other worktree identity error, got %v", err)
	}
	if gh.writeCalls() != 0 {
		t.Fatalf("invalid other worktree triggered %d writes", gh.writeCalls())
	}
}

type committedCreateErrorGitHub struct {
	*fakeGitHub
}

func (g committedCreateErrorGitHub) CreateIssue(ctx context.Context, repo, title, body string, labels []string) (Issue, error) {
	if _, err := g.fakeGitHub.CreateIssue(ctx, repo, title, body, labels); err != nil {
		return Issue{}, err
	}
	return Issue{}, errors.New("synthetic response lost after issue committed")
}

func TestAppMirrorRestartRecoveryAndNoop(t *testing.T) {
	for _, prefix := range []string{"task", "bug"} {
		for _, interrupted := range []bool{false, true} {
			name := prefix + "/healthy"
			if interrupted {
				name = prefix + "/committed-create-response-lost"
			}
			t.Run(name, func(t *testing.T) {
				root := tempRoot(t)
				task := Task{ID: strings.ToUpper(prefix) + "-1.2", Title: "Synthetic done task", Status: "Done"}
				bl := prefixOutputBacklog{fakeBacklog: newFakeBacklog(root, task), output: strings.ToUpper(prefix) + "\n"}
				gh := basicGH(map[string][]Issue{"owner/repo": {}})
				git := fakeGit{worktrees: []Worktree{{Path: root, Branch: "main", IsRoot: true}}}
				app := App{Git: git, Backlog: bl, GitHub: gh}
				if interrupted {
					app.GitHub = committedCreateErrorGitHub{gh}
				}
				c, err := app.Run(context.Background(), testConfig(root))
				if (err != nil) != interrupted || len(gh.issues["owner/repo"]) != 1 {
					t.Fatalf("first run: counters=%+v err=%v issues=%d", c, err, len(gh.issues["owner/repo"]))
				}
				if interrupted {
					if c.Failed != 1 || len(gh.items) != 0 || gh.addProjectItemCalls != 0 {
						t.Fatalf("interrupted creation must not attach a project item: counters=%+v items=%d", c, len(gh.items))
					}
				} else if c.Created != 1 || c.ProjectAdded != 1 || len(gh.items) != 1 {
					t.Fatalf("healthy create must attach its project item: counters=%+v items=%d", c, len(gh.items))
				}
				original := gh.issues["owner/repo"][0]
				if id, ok := ParseMarkerWithPrefix(original.Body, prefix); !ok || id != CanonicalTaskID(task.ID) {
					t.Fatalf("committed body lacks recovery marker: %q", original.Body)
				}

				// A new App has no run-local memory; recovery relies on the issue inventory.
				gh.resetWriteCounters()
				app = App{Git: git, Backlog: bl, GitHub: gh}
				c, err = app.Run(context.Background(), testConfig(root))
				if err != nil || c.Created != 0 || len(gh.createdIssues) != 0 || len(gh.issues["owner/repo"]) != 1 {
					t.Fatalf("restart must reuse committed issue: counters=%+v err=%v issues=%d", c, err, len(gh.issues["owner/repo"]))
				}
				issue := gh.issues["owner/repo"][0]
				if issue.NodeID != original.NodeID || issue.Number != original.Number || issue.State != "closed" ||
					len(gh.items) != 1 || gh.items[0].ContentNodeID != issue.NodeID || gh.items[0].Status != "Done" {
					t.Fatalf("restart did not converge original issue/project: issue=%+v items=%+v", issue, gh.items)
				}
				if interrupted && c.ProjectAdded != 1 {
					t.Fatalf("restart must add the missing project item: counters=%+v", c)
				}
				if !interrupted && gh.writeCalls() != 0 {
					t.Fatalf("healthy repeat must be a no-op, writes=%d", gh.writeCalls())
				}
				gh.resetWriteCounters()
				app = App{Git: git, Backlog: bl, GitHub: gh}
				c, err = app.Run(context.Background(), testConfig(root))
				if err != nil || c != (Counters{}) || gh.writeCalls() != 0 || len(bl.created) != 0 || bl.pushCount != 0 {
					t.Fatalf("converged restart must be a no-op: counters=%+v err=%v writes=%d", c, err, gh.writeCalls())
				}
			})
		}
	}
}

type inventoryOutputGitHub struct {
	*fakeGitHub
	output string
}

func (g inventoryOutputGitHub) ListIssues(ctx context.Context, repo string) ([]Issue, error) {
	return (ExecGitHub{AllowedRepos: []string{repo}, Runner: &scriptRunner{outputs: map[string][]byte{
		"gh api --paginate --slurp repos/" + repo + "/issues?state=all&per_page=100": []byte(g.output),
	}}}).ListIssues(ctx, repo)
}

func TestAppInvalidIssueInventoryAbortsBeforeWrites(t *testing.T) {
	for _, output := range []string{
		`null`, `[]`, `[null]`, `[[],null]`, `[[null]]`, `[[{}]]`,
		`[[{"number":1,"id":1}]]`,
		`[[{"number":1,"node_id":"I_1"}]]`,
		`[[{"id":1,"node_id":"I_1"}]]`,
	} {
		t.Run(output, func(t *testing.T) {
			root := tempRoot(t)
			bl := newFakeBacklog(root, Task{ID: "TASK-1", Title: "Synthetic task", Status: "To Do"})
			gh := inventoryOutputGitHub{fakeGitHub: basicGH(map[string][]Issue{"owner/repo": {}}), output: output}
			app := App{Git: fakeGit{worktrees: []Worktree{{Path: root, Branch: "main", IsRoot: true}}}, Backlog: bl, GitHub: gh}
			c, err := app.Run(context.Background(), testConfig(root))
			if err == nil || !strings.Contains(err.Error(), "issue inventory") {
				t.Errorf("invalid inventory must fail closed: counters=%+v err=%v", c, err)
			}
			if gh.writeCalls() != 0 || len(bl.created) != 0 || bl.pushCount != 0 {
				t.Fatalf("invalid inventory wrote: github=%d backlog=%d pushes=%d", gh.writeCalls(), len(bl.created), bl.pushCount)
			}
		})
	}
}

func TestExecIssueInventoryEmptyAndPaginated(t *testing.T) {
	for _, tc := range []struct {
		name, output string
		want         int
	}{
		{"empty repository", `[[]]`, 0},
		{"all states and PR filtering", `[[{"number":2,"id":2,"node_id":"I_2","state":"open","body":"<!-- backlog:task-1 -->"},{"number":3,"pull_request":{}}],[{"number":1,"id":1,"node_id":"I_1","state":"closed","body":"<!-- backlog:task-1 -->"}]]`, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := inventoryOutputGitHub{output: tc.output}
			issues, err := g.ListIssues(context.Background(), "owner/repo")
			if err != nil || len(issues) != tc.want {
				t.Fatalf("inventory: issues=%+v err=%v", issues, err)
			}
			if tc.want != 0 {
				indexed, _ := IndexIssues(issues, "inbox", "task", func(string, ...any) {})
				if indexed["task-1"].Number != 1 || indexed["task-1"].State != "closed" {
					t.Fatalf("lowest existing issue must win even when closed: %+v", indexed)
				}
			}
		})
	}
}
