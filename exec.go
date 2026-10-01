package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type CommandRunner interface {
	Run(ctx context.Context, dir string, name string, args []string, stdin []byte) ([]byte, error)
}

type OSCommandRunner struct{}

func (OSCommandRunner) Run(ctx context.Context, dir string, name string, args []string, stdin []byte) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.Bytes(), fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

type ExecBacklog struct{ Runner CommandRunner }

func (b ExecBacklog) ListTasks(ctx context.Context, dir string, maxCount, skip int) (TaskListResponse, error) {
	args := []string{"task", "list", "--json", "--max-count", strconv.Itoa(maxCount), "--skip", strconv.Itoa(skip)}
	out, err := b.Runner.Run(ctx, dir, "backlog", args, nil)
	if err != nil {
		return TaskListResponse{}, err
	}
	var resp TaskListResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return TaskListResponse{}, err
	}
	return resp, nil
}

func (b ExecBacklog) ViewTask(ctx context.Context, dir, id string) (TaskViewResponse, error) {
	out, err := b.Runner.Run(ctx, dir, "backlog", []string{"task", "view", id, "--json"}, nil)
	if err != nil {
		return TaskViewResponse{}, err
	}
	var resp TaskViewResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return TaskViewResponse{}, err
	}
	return resp, nil
}

func (b ExecBacklog) Statuses(ctx context.Context, dir string) ([]string, error) {
	out, err := b.Runner.Run(ctx, dir, "backlog", []string{"config", "get", "statuses"}, nil)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(strings.TrimSpace(string(out)), ",")
	statuses := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			statuses = append(statuses, part)
		}
	}
	if len(statuses) == 0 {
		return nil, errors.New("no statuses returned")
	}
	return statuses, nil
}

func (b ExecBacklog) CreateTask(ctx context.Context, dir, title, description string, labels []string) (string, error) {
	args := []string{"task", "create", title, "--plain", "-d", description}
	for _, label := range labels {
		args = append(args, "-l", label)
	}
	out, err := b.Runner.Run(ctx, dir, "backlog", args, nil)
	if err != nil {
		return "", err
	}
	match := regexp.MustCompile(`(?i)\bTASK-[0-9]+(?:\.[0-9]+)*\b`).FindString(string(out))
	if match == "" {
		return "", errors.New("created task response did not include task id")
	}
	return strings.ToUpper(match), nil
}

type ExecGit struct{ Runner CommandRunner }

func (g ExecGit) Worktrees(ctx context.Context, root string) ([]Worktree, error) {
	out, err := g.Runner.Run(ctx, "", "git", []string{"-C", root, "worktree", "list", "--porcelain"}, nil)
	if err != nil {
		return nil, err
	}
	return ParseWorktrees(string(out), filepath.Clean(root)), nil
}

func (g ExecGit) RootBranchClean(ctx context.Context, root string) (bool, string, error) {
	branchOut, err := g.Runner.Run(ctx, "", "git", []string{"-C", root, "branch", "--show-current"}, nil)
	if err != nil {
		return false, "", err
	}
	branch := strings.TrimSpace(string(branchOut))
	if branch != "main" {
		return false, fmt.Sprintf("root branch is %q, not main", branch), nil
	}
	for _, sentinel := range []string{"MERGE_HEAD", "rebase-merge", "rebase-apply", "CHERRY_PICK_HEAD"} {
		out, err := g.Runner.Run(ctx, "", "git", []string{"-C", root, "rev-parse", "--git-path", sentinel}, nil)
		if err != nil {
			return false, "", err
		}
		path := strings.TrimSpace(string(out))
		if path == "" {
			continue
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		if fileExists(path) {
			return false, fmt.Sprintf("git operation in progress: %s", sentinel), nil
		}
	}
	return true, "", nil
}

func ParseWorktrees(input string, root string) []Worktree {
	var out []Worktree
	var current *Worktree
	flush := func() {
		if current != nil {
			current.IsRoot = filepath.Clean(current.Path) == filepath.Clean(root)
			out = append(out, *current)
		}
		current = nil
	}
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			flush()
			continue
		}
		key, value, _ := strings.Cut(line, " ")
		switch key {
		case "worktree":
			flush()
			current = &Worktree{Path: value}
		case "branch":
			if current != nil {
				current.Branch = strings.TrimPrefix(value, "refs/heads/")
			}
		case "bare":
			if current != nil {
				current.Bare = true
			}
		case "prunable":
			if current != nil {
				current.Prunable = true
			}
		case "detached":
			if current != nil && current.Branch == "" {
				current.Branch = "(detached)"
			}
		}
	}
	flush()
	return out
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

type ExecGitHub struct{ Runner CommandRunner }

func (g ExecGitHub) ListIssues(ctx context.Context, repo string) ([]Issue, error) {
	path := fmt.Sprintf("repos/%s/issues?state=all&per_page=100", repo)
	out, err := g.Runner.Run(ctx, "", "gh", []string{"api", "--paginate", "--slurp", path}, nil)
	if err != nil {
		return nil, err
	}
	var pages [][]Issue
	if err := json.Unmarshal(out, &pages); err != nil {
		return nil, err
	}
	var issues []Issue
	for _, page := range pages {
		for _, issue := range page {
			if issue.PullRequest == nil {
				issues = append(issues, issue)
			}
		}
	}
	return issues, nil
}

func (g ExecGitHub) CreateIssue(ctx context.Context, repo string, title string, body string, labels []string) (Issue, error) {
	payload := map[string]any{"title": title, "body": body, "labels": labels}
	out, err := g.ghJSON(ctx, []string{"api", "-X", "POST", fmt.Sprintf("repos/%s/issues", repo), "--input", "-"}, payload)
	if err != nil {
		return Issue{}, err
	}
	var issue Issue
	if err := json.Unmarshal(out, &issue); err != nil {
		return Issue{}, err
	}
	return issue, nil
}

func (g ExecGitHub) UpdateIssue(ctx context.Context, repo string, number int, patch IssuePatch) (Issue, error) {
	payload := map[string]any{}
	if patch.Title != nil {
		payload["title"] = *patch.Title
	}
	if patch.Body != nil {
		payload["body"] = *patch.Body
	}
	if patch.Labels != nil {
		payload["labels"] = *patch.Labels
	}
	if patch.State != nil {
		payload["state"] = *patch.State
	}
	if patch.StateReason != nil {
		payload["state_reason"] = *patch.StateReason
	}
	out, err := g.ghJSON(ctx, []string{"api", "-X", "PATCH", fmt.Sprintf("repos/%s/issues/%d", repo, number), "--input", "-"}, payload)
	if err != nil {
		return Issue{}, err
	}
	var issue Issue
	if err := json.Unmarshal(out, &issue); err != nil {
		return Issue{}, err
	}
	return issue, nil
}

func (g ExecGitHub) EnsureLabel(ctx context.Context, repo, label string) error {
	if label == "" {
		return nil
	}
	escaped := url.PathEscape(label)
	_, err := g.Runner.Run(ctx, "", "gh", []string{"api", fmt.Sprintf("repos/%s/labels/%s", repo, escaped)}, nil)
	if err == nil {
		return nil
	}
	_, err = g.ghJSON(ctx, []string{"api", "-X", "POST", fmt.Sprintf("repos/%s/labels", repo), "--input", "-"}, map[string]any{"name": label, "color": "ededed"})
	return err
}

func (g ExecGitHub) ProjectInfo(ctx context.Context, owner string, number int) (ProjectInfo, error) {
	query := `query($owner:String!, $number:Int!) { user(login:$owner) { projectV2(number:$number) { id fields(first:100) { nodes { ... on ProjectV2SingleSelectField { id name options { id name } } } } } } }`
	payload := map[string]any{"query": query, "variables": map[string]any{"owner": owner, "number": number}}
	out, err := g.ghJSON(ctx, []string{"api", "graphql", "--input", "-"}, payload)
	if err != nil {
		return ProjectInfo{}, err
	}
	var resp struct {
		Data struct {
			User struct {
				ProjectV2 struct {
					ID     string `json:"id"`
					Fields struct {
						Nodes []struct {
							ID      string `json:"id"`
							Name    string `json:"name"`
							Options []struct {
								ID   string `json:"id"`
								Name string `json:"name"`
							} `json:"options"`
						} `json:"nodes"`
					} `json:"fields"`
				} `json:"projectV2"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return ProjectInfo{}, err
	}
	info := ProjectInfo{ID: resp.Data.User.ProjectV2.ID, StatusOptionID: map[string]string{}}
	for _, field := range resp.Data.User.ProjectV2.Fields.Nodes {
		if field.Name != "Status" {
			continue
		}
		info.StatusFieldID = field.ID
		for _, option := range field.Options {
			info.StatusOptionID[option.Name] = option.ID
		}
	}
	if info.ID == "" || info.StatusFieldID == "" {
		return ProjectInfo{}, errors.New("project or Status field not found")
	}
	return info, nil
}

func (g ExecGitHub) ListProjectItems(ctx context.Context, projectID string) ([]ProjectItem, error) {
	query := `query($project:ID!, $after:String) { node(id:$project) { ... on ProjectV2 { items(first:100, after:$after) { nodes { id content { ... on Issue { id } } fieldValueByName(name:"Status") { ... on ProjectV2ItemFieldSingleSelectValue { name optionId } } } pageInfo { hasNextPage endCursor } } } } }`
	var items []ProjectItem
	var after any
	for {
		payload := map[string]any{"query": query, "variables": map[string]any{"project": projectID, "after": after}}
		out, err := g.ghJSON(ctx, []string{"api", "graphql", "--input", "-"}, payload)
		if err != nil {
			return nil, err
		}
		var resp struct {
			Data struct {
				Node struct {
					Items struct {
						Nodes []struct {
							ID      string `json:"id"`
							Content *struct {
								ID string `json:"id"`
							} `json:"content"`
							FieldValueByName *struct {
								Name     string `json:"name"`
								OptionID string `json:"optionId"`
							} `json:"fieldValueByName"`
						} `json:"nodes"`
						PageInfo struct {
							HasNextPage bool    `json:"hasNextPage"`
							EndCursor   *string `json:"endCursor"`
						} `json:"pageInfo"`
					} `json:"items"`
				} `json:"node"`
			} `json:"data"`
		}
		if err := json.Unmarshal(out, &resp); err != nil {
			return nil, err
		}
		for _, node := range resp.Data.Node.Items.Nodes {
			item := ProjectItem{ID: node.ID}
			if node.Content != nil {
				item.ContentNodeID = node.Content.ID
			}
			if node.FieldValueByName != nil {
				item.Status = node.FieldValueByName.Name
				item.StatusOptionID = node.FieldValueByName.OptionID
			}
			items = append(items, item)
		}
		if !resp.Data.Node.Items.PageInfo.HasNextPage || resp.Data.Node.Items.PageInfo.EndCursor == nil {
			break
		}
		after = *resp.Data.Node.Items.PageInfo.EndCursor
	}
	return items, nil
}

func (g ExecGitHub) AddProjectItem(ctx context.Context, projectID, contentNodeID string) (ProjectItem, error) {
	query := `mutation($project:ID!, $content:ID!) { addProjectV2ItemById(input:{projectId:$project, contentId:$content}) { item { id } } }`
	payload := map[string]any{"query": query, "variables": map[string]any{"project": projectID, "content": contentNodeID}}
	out, err := g.ghJSON(ctx, []string{"api", "graphql", "--input", "-"}, payload)
	if err != nil {
		return ProjectItem{}, err
	}
	var resp struct {
		Data struct {
			AddProjectV2ItemByID struct {
				Item ProjectItem `json:"item"`
			} `json:"addProjectV2ItemById"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return ProjectItem{}, err
	}
	return resp.Data.AddProjectV2ItemByID.Item, nil
}

func (g ExecGitHub) UpdateProjectStatus(ctx context.Context, projectID, itemID, fieldID, optionID string) error {
	query := `mutation($project:ID!, $item:ID!, $field:ID!, $option:String!) { updateProjectV2ItemFieldValue(input:{projectId:$project, itemId:$item, fieldId:$field, value:{singleSelectOptionId:$option}}) { projectV2Item { id } } }`
	payload := map[string]any{"query": query, "variables": map[string]any{"project": projectID, "item": itemID, "field": fieldID, "option": optionID}}
	_, err := g.ghJSON(ctx, []string{"api", "graphql", "--input", "-"}, payload)
	return err
}

func (g ExecGitHub) ghJSON(ctx context.Context, args []string, payload any) ([]byte, error) {
	input, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return g.Runner.Run(ctx, "", "gh", args, input)
}
