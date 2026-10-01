package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Backlog interface {
	ListTasks(ctx context.Context, dir string, maxCount, skip int) (TaskListResponse, error)
	ViewTask(ctx context.Context, dir, id string) (TaskViewResponse, error)
	Statuses(ctx context.Context, dir string) ([]string, error)
	TaskPrefix(ctx context.Context, dir string) (string, error)
	Milestones(ctx context.Context, dir string) (map[string]string, error)
	CreateTask(ctx context.Context, dir string, in CreateTaskInput) (string, error)
	Push(ctx context.Context, dir, branch string) error
}

type Git interface {
	Worktrees(ctx context.Context, root string) ([]Worktree, error)
	RootBranchClean(ctx context.Context, root, mainBranch string) (bool, string, error)
	RemoteRepo(ctx context.Context, root, remote string) (string, error)
	Fetch(ctx context.Context, root, remote, branch string) error
	AddWorktree(ctx context.Context, root, path, branch, startPoint string) error
	RemoveWorktree(ctx context.Context, root, path string) error
	PushBranch(ctx context.Context, dir, branch string) error
}

type GitHub interface {
	ListIssues(ctx context.Context, repo string) ([]Issue, error)
	CreateIssue(ctx context.Context, repo string, title string, body string, labels []string) (Issue, error)
	UpdateIssue(ctx context.Context, repo string, number int, patch IssuePatch) (Issue, error)
	EnsureLabel(ctx context.Context, repo, label string) error
	ProjectInfo(ctx context.Context, ownerType, owner string, number int) (ProjectInfo, error)
	ListProjectItems(ctx context.Context, projectID string) ([]ProjectItem, error)
	AddProjectItem(ctx context.Context, projectID, contentNodeID string) (ProjectItem, error)
	UpdateProjectStatus(ctx context.Context, projectID, itemID, fieldID, optionID string) error
	UpdateProjectSingleSelect(ctx context.Context, projectID, itemID, fieldID, optionID string) error
	UpdateProjectText(ctx context.Context, projectID, itemID, fieldID, text string) error
	ClearProjectField(ctx context.Context, projectID, itemID, fieldID string) error
	IssueParent(ctx context.Context, issueNodeID string) (IssueParentInfo, error)
	AddSubIssue(ctx context.Context, parentRepo string, parentNumber int, childDatabaseID int64) error
	RemoveSubIssue(ctx context.Context, parentNodeID string, childNodeID string) error
	ListOpenPullRequests(ctx context.Context, repo string) ([]PullRequest, error)
	CreatePullRequest(ctx context.Context, repo, head, base, title, body string) (PullRequest, error)
}

func CollectTasks(ctx context.Context, cfg Config, git Git, backlog Backlog, logf func(string, ...any)) (ResolvedTasks, error) {
	root := cfg.Root
	worktrees, err := git.Worktrees(ctx, root)
	if err != nil {
		return ResolvedTasks{}, err
	}
	copies := make(map[string][]TaskCopy)
	mainTaskCount := -1
	for _, wt := range worktrees {
		if shouldSkipWorktree(wt) {
			continue
		}
		if dir, ok, err := DiscoverBacklogDir(wt.Path); err != nil {
			return ResolvedTasks{}, fmt.Errorf("discover Backlog directory in %s: %w", wt.Path, err)
		} else if !ok {
			if logf != nil {
				logf("skip worktree without Backlog config: %s", wt.Path)
			}
			continue
		} else if logf != nil {
			logf("scan Backlog directory %s", dir)
		}
		summaries, err := listAllTasks(ctx, backlog, wt.Path)
		if err != nil {
			return ResolvedTasks{}, fmt.Errorf("list tasks in %s: %w", wt.Path, err)
		}
		if wt.IsRoot {
			mainTaskCount = len(summaries)
		}
		for _, summary := range summaries {
			view, err := backlog.ViewTask(ctx, wt.Path, summary.ID)
			if err != nil {
				return ResolvedTasks{}, fmt.Errorf("view %s in %s: %w", summary.ID, wt.Path, err)
			}
			task := view.Task
			task.ID = UpperTaskID(task.ID)
			task.Branch = wt.Branch
			task.WorktreePath = wt.Path
			copies[CanonicalTaskID(task.ID)] = append(copies[CanonicalTaskID(task.ID)], TaskCopy{Task: task, Worktree: wt})
		}
	}
	if mainTaskCount == 0 {
		return ResolvedTasks{}, fmt.Errorf("safety abort: main worktree %s returned 0 tasks", root)
	}
	if mainTaskCount < 0 {
		return ResolvedTasks{}, fmt.Errorf("safety abort: main worktree %s was not scanned", root)
	}
	statuses, err := backlog.Statuses(ctx, root)
	if err != nil {
		return ResolvedTasks{}, fmt.Errorf("read backlog statuses: %w", err)
	}
	statusRank := make(map[string]int, len(statuses))
	for i, status := range statuses {
		statusRank[status] = i
	}
	resolved := ResolvedTasks{ByID: map[string]Task{}, BranchByID: map[string]string{}}
	ids := make([]string, 0, len(copies))
	for id := range copies {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		winner := ResolveTaskCopyWithPrefix(id, copies[id], statusRank, root, cfg.TaskPrefix)
		resolved.Tasks = append(resolved.Tasks, winner.Task)
		resolved.ByID[id] = winner.Task
		resolved.BranchByID[id] = winner.Worktree.Branch
	}
	return resolved, nil
}

func listAllTasks(ctx context.Context, backlog Backlog, dir string) ([]TaskSummary, error) {
	const pageSize = 100
	var out []TaskSummary
	skip := 0
	for {
		page, err := backlog.ListTasks(ctx, dir, pageSize, skip)
		if err != nil {
			return nil, err
		}
		out = append(out, page.Tasks...)
		if page.Total == nil {
			if page.NextSkip != nil || len(page.Tasks) == pageSize {
				return nil, fmt.Errorf("pagination response missing total")
			}
			break
		}
		if page.NextSkip == nil {
			if len(out) != *page.Total {
				return nil, fmt.Errorf("pagination ended with %d tasks but total is %d", len(out), *page.Total)
			}
			break
		}
		if *page.NextSkip <= skip {
			return nil, fmt.Errorf("invalid nextSkip %d after skip %d", *page.NextSkip, skip)
		}
		skip = *page.NextSkip
	}
	return out, nil
}

func shouldSkipWorktree(wt Worktree) bool {
	if wt.Path == "" || wt.Bare || wt.Prunable || wt.Missing {
		return true
	}
	if _, err := os.Stat(wt.Path); err != nil {
		return true
	}
	return false
}

func DiscoverBacklogDir(root string) (string, bool, error) {
	for _, rel := range []string{"backlog", ".backlog"} {
		path := filepath.Join(root, rel)
		if isBacklogDataDir(path) {
			return path, true, nil
		}
	}
	var found []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == root {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		if entry.IsDir() {
			name := entry.Name()
			if name == ".git" || name == "node_modules" || name == ".build" || name == "DerivedData" {
				return filepath.SkipDir
			}
			if strings.Count(rel, string(os.PathSeparator)) >= 3 {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() != "config.yml" {
			return nil
		}
		dir := filepath.Dir(path)
		if isBacklogDataDir(dir) {
			found = append(found, dir)
		}
		return nil
	})
	if err != nil {
		return "", false, err
	}
	sort.Strings(found)
	unique := found[:0]
	for _, dir := range found {
		if len(unique) == 0 || unique[len(unique)-1] != dir {
			unique = append(unique, dir)
		}
	}
	if len(unique) == 0 {
		return "", false, nil
	}
	if len(unique) > 1 {
		return "", false, fmt.Errorf("multiple Backlog data directories found: %s", strings.Join(unique, ", "))
	}
	return unique[0], true, nil
}

func isBacklogDataDir(path string) bool {
	info, err := os.Stat(filepath.Join(path, "config.yml"))
	if err != nil || info.IsDir() {
		return false
	}
	if taskInfo, err := os.Stat(filepath.Join(path, "tasks")); err == nil && taskInfo.IsDir() {
		return true
	}
	data, err := os.ReadFile(filepath.Join(path, "config.yml"))
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "task_prefix:") || strings.Contains(string(data), "statuses:")
}

func ResolveTaskCopy(id string, copies []TaskCopy, statusRank map[string]int, root string) TaskCopy {
	return ResolveTaskCopyWithPrefix(id, copies, statusRank, root, "task")
}

func ResolveTaskCopyWithPrefix(id string, copies []TaskCopy, statusRank map[string]int, root, taskPrefix string) TaskCopy {
	if len(copies) == 0 {
		return TaskCopy{}
	}
	for _, copy := range copies {
		if BranchOwnsTaskWithPrefix(id, copy.Worktree.Branch, taskPrefix) {
			return copy
		}
	}
	sort.SliceStable(copies, func(i, j int) bool {
		left, right := copies[i], copies[j]
		lt, rt := taskTimestamp(left.Task), taskTimestamp(right.Task)
		if !lt.Equal(rt) {
			return lt.After(rt)
		}
		lr, rr := statusRankValue(statusRank, left.Task.Status), statusRankValue(statusRank, right.Task.Status)
		if lr != rr {
			return lr > rr
		}
		if left.Worktree.IsRoot != right.Worktree.IsRoot {
			return left.Worktree.IsRoot
		}
		if left.Worktree.Path == root && right.Worktree.Path != root {
			return true
		}
		if left.Worktree.Path != root && right.Worktree.Path == root {
			return false
		}
		return left.Worktree.Path < right.Worktree.Path
	})
	return copies[0]
}

func BranchOwnsTask(id, branch string) bool { return BranchOwnsTaskWithPrefix(id, branch, "task") }

func BranchOwnsTaskWithPrefix(id, branch, taskPrefix string) bool {
	id = CanonicalTaskID(id)
	branch = strings.ToLower(branch)
	if !taskIDRegexp(taskPrefix).MatchString(id) {
		return false
	}
	return branch == id || strings.HasPrefix(branch, id+"-")
}

func taskTimestamp(task Task) time.Time {
	if task.UpdatedAt != nil {
		return *task.UpdatedAt
	}
	if task.CreatedAt != nil {
		return *task.CreatedAt
	}
	return time.Time{}
}

func statusRankValue(rank map[string]int, status string) int {
	if value, ok := rank[status]; ok {
		return value
	}
	return -1
}
