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
	"time"
)

type CommandRunner interface {
	Run(ctx context.Context, dir string, name string, args []string, stdin []byte) ([]byte, error)
}

type OSCommandRunner struct{ Timeout time.Duration }

func (r OSCommandRunner) Run(ctx context.Context, dir string, name string, args []string, stdin []byte) ([]byte, error) {
	if r.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.Timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = scrubBacklogEnv(os.Environ())
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return stdout.Bytes(), fmt.Errorf("%s %s: timed out after %s", name, strings.Join(args, " "), r.Timeout)
	}
	if err != nil {
		return stdout.Bytes(), fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

func scrubBacklogEnv(in []string) []string {
	out := make([]string, 0, len(in))
	for _, env := range in {
		key, _, _ := strings.Cut(env, "=")
		if key == "BACKLOG_CWD" || strings.HasPrefix(key, "BACKLOG_") {
			continue
		}
		out = append(out, env)
	}
	return out
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

func (b ExecBacklog) TaskPrefix(ctx context.Context, dir string) (string, error) {
	for _, key := range []string{"taskPrefix", "task_prefix"} {
		out, err := b.Runner.Run(ctx, dir, "backlog", []string{"config", "get", key}, nil)
		if err == nil {
			prefix := strings.TrimSpace(string(out))
			if prefix != "" {
				return strings.ToLower(prefix), nil
			}
		}
	}
	if prefix, ok, err := readTaskPrefixFromRootConfig(dir); err != nil || ok {
		return strings.ToLower(strings.TrimSpace(prefix)), err
	}
	backlogDir, ok, err := DiscoverBacklogDir(dir)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", errors.New("Backlog data directory not found")
	}
	prefix, ok, err := readTaskPrefixFromBacklogDir(backlogDir)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("task_prefix not found in %s or %s", filepath.Join(dir, rootBacklogConfigFile), backlogDir)
	}
	return strings.ToLower(strings.TrimSpace(prefix)), nil
}

func readTaskPrefixFromRootConfig(root string) (string, bool, error) {
	configPath := filepath.Join(root, rootBacklogConfigFile)
	data, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read %s: %w", configPath, err)
	}
	prefix, ok, err := parseRootConfigValue(data, "task_prefix")
	if err != nil {
		return "", false, fmt.Errorf("parse %s: %w", configPath, err)
	}
	return prefix, ok, nil
}

func readTaskPrefixFromBacklogDir(backlogDir string) (string, bool, error) {
	configPath, ok := backlogDataConfigPath(backlogDir)
	if !ok {
		return "", false, nil
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", false, err
	}
	prefix, ok, err := parseRootConfigValue(data, "task_prefix")
	if err != nil {
		return "", false, fmt.Errorf("parse %s: %w", configPath, err)
	}
	return prefix, ok, nil
}

func (b ExecBacklog) Milestones(ctx context.Context, dir string) (map[string]string, error) {
	out, err := b.Runner.Run(ctx, dir, "backlog", []string{"milestone", "list", "--plain"}, nil)
	if err != nil {
		return nil, err
	}
	return ParseMilestones(string(out)), nil
}

func ParseMilestones(input string) map[string]string {
	out := map[string]string{}
	re := regexp.MustCompile(`^\s*(m-[^:]+):\s*(.*?)\s*(?:\([0-9]+/[0-9]+ done\))?\s*$`)
	for _, line := range strings.Split(input, "\n") {
		m := re.FindStringSubmatch(line)
		if len(m) == 3 {
			out[m[1]] = strings.TrimSpace(m[2])
		}
	}
	return out
}

func (b ExecBacklog) CreateTask(ctx context.Context, dir string, in CreateTaskInput) (string, error) {
	args := []string{"task", "create", "--plain", "-d", in.Description}
	if in.Project != "" {
		args = append(args, "--project", in.Project)
	}
	for _, ref := range in.References {
		args = append(args, "--ref", ref)
	}
	for _, label := range in.Labels {
		args = append(args, "-l", label)
	}
	args = append(args, "--", in.Title)
	out, err := b.Runner.Run(ctx, dir, "backlog", args, nil)
	if err != nil {
		return "", err
	}
	match := taskIDRegexp(in.TaskPrefix).FindString(string(out))
	if match == "" {
		return "", errors.New("created task response did not include task id")
	}
	return strings.ToUpper(match), nil
}

func (b ExecBacklog) Push(ctx context.Context, dir, branch string) error {
	_, err := b.Runner.Run(ctx, dir, "git", []string{"-C", dir, "push", "origin", branch}, nil)
	return err
}

type CreateTaskInput struct {
	Title       string
	Description string
	Labels      []string
	Project     string
	References  []string
	TaskPrefix  string
}

type ExecGit struct{ Runner CommandRunner }

func (g ExecGit) Worktrees(ctx context.Context, root string) ([]Worktree, error) {
	out, err := g.Runner.Run(ctx, "", "git", []string{"-C", root, "worktree", "list", "--porcelain"}, nil)
	if err != nil {
		return nil, err
	}
	return ParseWorktrees(string(out), filepath.Clean(root)), nil
}

func (g ExecGit) RootBranchClean(ctx context.Context, root, mainBranch string) (bool, string, error) {
	branchOut, err := g.Runner.Run(ctx, "", "git", []string{"-C", root, "branch", "--show-current"}, nil)
	if err != nil {
		return false, "", err
	}
	branch := strings.TrimSpace(string(branchOut))
	if branch != mainBranch {
		return false, fmt.Sprintf("root branch is %q, not %s", branch, mainBranch), nil
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
	if out, err := g.Runner.Run(ctx, "", "git", []string{"--no-optional-locks", "-C", root, "status", "--porcelain=v1"}, nil); err != nil {
		return false, "", err
	} else if strings.TrimSpace(string(out)) != "" {
		return false, "root worktree or index is dirty", nil
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

func fileExists(path string) bool { _, err := os.Stat(path); return err == nil }

type ExecGitHub struct {
	Runner       CommandRunner
	AllowedRepos []string
}

func (g ExecGitHub) checkRepo(repo string) error {
	for _, allowed := range g.AllowedRepos {
		if strings.EqualFold(repo, allowed) {
			return nil
		}
	}
	return fmt.Errorf("refusing GitHub access to unconfigured repo %s", repo)
}

func (g ExecGitHub) ListIssues(ctx context.Context, repo string) ([]Issue, error) {
	if err := g.checkRepo(repo); err != nil {
		return nil, err
	}
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
				issue.Repo = repo
				issues = append(issues, issue)
			}
		}
	}
	return issues, nil
}

func (g ExecGitHub) CreateIssue(ctx context.Context, repo string, title string, body string, labels []string) (Issue, error) {
	if err := g.checkRepo(repo); err != nil {
		return Issue{}, err
	}
	payload := map[string]any{"title": title, "body": body, "labels": labels}
	out, err := g.ghJSON(ctx, []string{"api", "-X", "POST", fmt.Sprintf("repos/%s/issues", repo), "--input", "-"}, payload)
	if err != nil {
		return Issue{}, err
	}
	var issue Issue
	if err := json.Unmarshal(out, &issue); err != nil {
		return Issue{}, err
	}
	issue.Repo = repo
	return issue, nil
}

func (g ExecGitHub) UpdateIssue(ctx context.Context, repo string, number int, patch IssuePatch) (Issue, error) {
	if err := g.checkRepo(repo); err != nil {
		return Issue{}, err
	}
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
	issue.Repo = repo
	return issue, nil
}

func (g ExecGitHub) EnsureLabel(ctx context.Context, repo, label string) error {
	if label == "" {
		return nil
	}
	if err := g.checkRepo(repo); err != nil {
		return err
	}
	escaped := url.PathEscape(label)
	_, err := g.Runner.Run(ctx, "", "gh", []string{"api", fmt.Sprintf("repos/%s/labels/%s", repo, escaped)}, nil)
	if err == nil {
		return nil
	}
	if !strings.Contains(err.Error(), "HTTP 404") && !strings.Contains(err.Error(), "Not Found") {
		return err
	}
	_, err = g.ghJSON(ctx, []string{"api", "-X", "POST", fmt.Sprintf("repos/%s/labels", repo), "--input", "-"}, map[string]any{"name": label, "color": "ededed"})
	return err
}

func (g ExecGitHub) ProjectInfo(ctx context.Context, ownerType, owner string, number int) (ProjectInfo, error) {
	ownerField := "user"
	if ownerType == "org" {
		ownerField = "organization"
	}
	query := fmt.Sprintf(`query($owner:String!, $number:Int!) { %s(login:$owner) { projectV2(number:$number) { id fields(first:100) { nodes { ... on ProjectV2Field { id name dataType } ... on ProjectV2SingleSelectField { id name options { id name } } } } } } }`, ownerField)
	payload := map[string]any{"query": query, "variables": map[string]any{"owner": owner, "number": number}}
	out, err := g.ghJSON(ctx, []string{"api", "graphql", "--input", "-"}, payload)
	if err != nil {
		return ProjectInfo{}, err
	}
	var resp struct {
		Data map[string]struct {
			ProjectV2 struct {
				ID     string `json:"id"`
				Fields struct {
					Nodes []struct {
						ID       string `json:"id"`
						Name     string `json:"name"`
						DataType string `json:"dataType"`
						Options  []struct {
							ID   string `json:"id"`
							Name string `json:"name"`
						} `json:"options"`
					} `json:"nodes"`
				} `json:"fields"`
			} `json:"projectV2"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return ProjectInfo{}, err
	}
	ownerNode := resp.Data[ownerField]
	info := ProjectInfo{ID: ownerNode.ProjectV2.ID, StatusOptionID: map[string]string{}, Fields: map[string]ProjectField{}}
	for _, field := range ownerNode.ProjectV2.Fields.Nodes {
		pf := ProjectField{ID: field.ID, Name: field.Name, DataType: field.DataType, Options: map[string]string{}}
		for _, option := range field.Options {
			pf.Options[option.Name] = option.ID
		}
		info.Fields[field.Name] = pf
		if field.Name == "Status" {
			info.StatusFieldID = field.ID
			for _, option := range field.Options {
				info.StatusOptionID[option.Name] = option.ID
			}
		}
	}
	if info.ID == "" || info.StatusFieldID == "" {
		return ProjectInfo{}, errors.New("project or Status field not found")
	}
	return info, nil
}

func (g ExecGitHub) ListProjectItems(ctx context.Context, projectID string) ([]ProjectItem, error) {
	query := `query($project:ID!, $after:String) { node(id:$project) { ... on ProjectV2 { items(first:100, after:$after) { nodes { id content { ... on Issue { id } } fieldValueByName(name:"Status") { ... on ProjectV2ItemFieldSingleSelectValue { name optionId } } fieldValues(first:100) { nodes { ... on ProjectV2ItemFieldSingleSelectValue { field { ... on ProjectV2FieldCommon { name } } name optionId } ... on ProjectV2ItemFieldTextValue { field { ... on ProjectV2FieldCommon { name } } text } } } } pageInfo { hasNextPage endCursor } } } } }`
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
							FieldValues struct {
								Nodes []struct {
									Field *struct {
										Name string `json:"name"`
									} `json:"field"`
									Name     string `json:"name"`
									OptionID string `json:"optionId"`
									Text     string `json:"text"`
								} `json:"nodes"`
							} `json:"fieldValues"`
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
			item := ProjectItem{ID: node.ID, FieldValues: map[string]ProjectFieldValue{}}
			if node.Content != nil {
				item.ContentNodeID = node.Content.ID
			}
			if node.FieldValueByName != nil {
				item.Status = node.FieldValueByName.Name
				item.StatusOptionID = node.FieldValueByName.OptionID
			}
			for _, fv := range node.FieldValues.Nodes {
				if fv.Field == nil {
					continue
				}
				item.FieldValues[fv.Field.Name] = ProjectFieldValue{Name: fv.Name, OptionID: fv.OptionID, Text: fv.Text}
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
	return g.UpdateProjectSingleSelect(ctx, projectID, itemID, fieldID, optionID)
}
func (g ExecGitHub) UpdateProjectSingleSelect(ctx context.Context, projectID, itemID, fieldID, optionID string) error {
	query := `mutation($project:ID!, $item:ID!, $field:ID!, $option:String!) { updateProjectV2ItemFieldValue(input:{projectId:$project, itemId:$item, fieldId:$field, value:{singleSelectOptionId:$option}}) { projectV2Item { id } } }`
	payload := map[string]any{"query": query, "variables": map[string]any{"project": projectID, "item": itemID, "field": fieldID, "option": optionID}}
	_, err := g.ghJSON(ctx, []string{"api", "graphql", "--input", "-"}, payload)
	return err
}
func (g ExecGitHub) UpdateProjectText(ctx context.Context, projectID, itemID, fieldID, text string) error {
	query := `mutation($project:ID!, $item:ID!, $field:ID!, $text:String!) { updateProjectV2ItemFieldValue(input:{projectId:$project, itemId:$item, fieldId:$field, value:{text:$text}}) { projectV2Item { id } } }`
	payload := map[string]any{"query": query, "variables": map[string]any{"project": projectID, "item": itemID, "field": fieldID, "text": text}}
	_, err := g.ghJSON(ctx, []string{"api", "graphql", "--input", "-"}, payload)
	return err
}
func (g ExecGitHub) ClearProjectField(ctx context.Context, projectID, itemID, fieldID string) error {
	query := `mutation($project:ID!, $item:ID!, $field:ID!) { clearProjectV2ItemFieldValue(input:{projectId:$project, itemId:$item, fieldId:$field}) { projectV2Item { id } } }`
	payload := map[string]any{"query": query, "variables": map[string]any{"project": projectID, "item": itemID, "field": fieldID}}
	_, err := g.ghJSON(ctx, []string{"api", "graphql", "--input", "-"}, payload)
	return err
}
func (g ExecGitHub) IssueParent(ctx context.Context, issueNodeID string) (IssueParentInfo, error) {
	query := `query($id:ID!) { node(id:$id) { ... on Issue { parent { id number repository { nameWithOwner } } } } }`
	payload := map[string]any{"query": query, "variables": map[string]any{"id": issueNodeID}}
	out, err := g.ghJSON(ctx, []string{"api", "graphql", "--input", "-"}, payload)
	if err != nil {
		return IssueParentInfo{}, err
	}
	var resp struct {
		Data struct {
			Node struct {
				Parent *struct {
					ID         string `json:"id"`
					Number     int    `json:"number"`
					Repository struct {
						NameWithOwner string `json:"nameWithOwner"`
					} `json:"repository"`
				} `json:"parent"`
			} `json:"node"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return IssueParentInfo{}, err
	}
	if resp.Data.Node.Parent == nil {
		return IssueParentInfo{}, nil
	}
	return IssueParentInfo{ID: resp.Data.Node.Parent.ID, Number: resp.Data.Node.Parent.Number, Repo: resp.Data.Node.Parent.Repository.NameWithOwner}, nil
}
func (g ExecGitHub) AddSubIssue(ctx context.Context, parentRepo string, parentNumber int, childDatabaseID int64) error {
	if err := g.checkRepo(parentRepo); err != nil {
		return err
	}
	_, err := g.ghJSON(ctx, []string{"api", "-X", "POST", fmt.Sprintf("repos/%s/issues/%d/sub_issues", parentRepo, parentNumber), "--input", "-"}, map[string]any{"sub_issue_id": childDatabaseID})
	return err
}
func (g ExecGitHub) RemoveSubIssue(ctx context.Context, parentNodeID string, childNodeID string) error {
	query := `mutation($parent:ID!, $child:ID!) { removeSubIssue(input:{issueId:$parent, subIssueId:$child}) { issue { id } } }`
	payload := map[string]any{"query": query, "variables": map[string]any{"parent": parentNodeID, "child": childNodeID}}
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
