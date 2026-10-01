package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

const mirrorNotice = "Mirrored one-way from Backlog.md — edits here are overwritten. Change it with the backlog CLI."

type Config struct {
	Root                  string            `json:"root"`
	ConfigPath            string            `json:"-"`
	ProjectOwner          string            `json:"projectOwner"`
	ProjectOwnerType      string            `json:"projectOwnerType"`
	ProjectNumber         int               `json:"projectNumber"`
	Repos                 map[string]string `json:"repos"`
	DefaultRepo           string            `json:"defaultRepo"`
	MainBranch            string            `json:"mainBranch"`
	StatusMap             map[string]string `json:"statusMap"`
	Inbox                 InboxConfig       `json:"inbox"`
	AdoptReferencedIssues bool              `json:"adoptReferencedIssues"`
	Labels                LabelConfig       `json:"labels"`
	Fields                FieldConfig       `json:"fields"`
	SubIssues             bool              `json:"subIssues"`
	LockFile              string            `json:"lockFile"`
	TimeoutSeconds        int               `json:"timeoutSeconds"`
	DryRun                bool              `json:"-"`
	NoInbox               bool              `json:"-"`
	Verbose               bool              `json:"-"`
	TaskPrefix            string            `json:"-"`
}

type InboxConfig struct {
	Enabled bool   `json:"enabled"`
	Label   string `json:"label"`
	Push    bool   `json:"push"`
}

type LabelConfig struct {
	Managed        []string `json:"managed"`
	AddAlways      []string `json:"addAlways"`
	PriorityPrefix string   `json:"priorityPrefix"`
}

type FieldConfig struct {
	Priority  string `json:"priority"`
	Milestone string `json:"milestone"`
	Area      string `json:"area"`
	TaskID    string `json:"taskId"`
	Branch    string `json:"branch"`
}

type Worktree struct {
	Path     string
	Branch   string
	Bare     bool
	Prunable bool
	Missing  bool
	IsRoot   bool
}

type TaskSummary struct {
	ID           string     `json:"id"`
	Title        string     `json:"title"`
	Status       string     `json:"status"`
	Labels       []string   `json:"labels"`
	Milestone    *string    `json:"milestone"`
	ParentTaskID *string    `json:"parentTaskId"`
	Assignees    []string   `json:"assignees"`
	CreatedAt    *time.Time `json:"createdAt"`
	UpdatedAt    *time.Time `json:"updatedAt"`
}

type TaskListResponse struct {
	Tasks    []TaskSummary `json:"tasks"`
	Total    *int          `json:"total"`
	NextSkip *int          `json:"nextSkip"`
}

type TaskViewResponse struct {
	Task Task `json:"task"`
}

type Task struct {
	ID                  string                `json:"id"`
	Title               string                `json:"title"`
	Status              string                `json:"status"`
	Type                *string               `json:"type"`
	Priority            *string               `json:"priority"`
	Project             *string               `json:"project"`
	Labels              []string              `json:"labels"`
	Milestone           *string               `json:"milestone"`
	ParentTaskID        *string               `json:"parentTaskId"`
	Assignees           []string              `json:"assignees"`
	CreatedAt           *time.Time            `json:"createdAt"`
	UpdatedAt           *time.Time            `json:"updatedAt"`
	Description         string                `json:"description"`
	Dependencies        []string              `json:"dependencies"`
	Subtasks            []TaskRef             `json:"subtasks"`
	References          []string              `json:"references"`
	AcceptanceCriteria  []AcceptanceCriterion `json:"acceptanceCriteria"`
	ImplementationPlan  string                `json:"implementationPlan"`
	ImplementationNotes string                `json:"implementationNotes"`
	FinalSummary        *string               `json:"finalSummary"`
	Branch              string                `json:"-"`
	WorktreePath        string                `json:"-"`
}

type TaskRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type AcceptanceCriterion struct {
	Index   int    `json:"index"`
	Text    string `json:"text"`
	Checked bool   `json:"checked"`
}

type Issue struct {
	Number      int          `json:"number"`
	DatabaseID  int64        `json:"id"`
	NodeID      string       `json:"node_id"`
	HTMLURL     string       `json:"html_url"`
	Title       string       `json:"title"`
	Body        string       `json:"body"`
	State       string       `json:"state"`
	StateReason string       `json:"state_reason"`
	Labels      []IssueLabel `json:"labels"`
	PullRequest *struct{}    `json:"pull_request"`
	Repo        string       `json:"-"`
}

type IssueLabel struct {
	Name string `json:"name"`
}

type IssuePatch struct {
	Title       *string
	Body        *string
	Labels      *[]string
	State       *string
	StateReason *string
}

type ProjectInfo struct {
	ID             string
	StatusFieldID  string
	StatusOptionID map[string]string
	Fields         map[string]ProjectField
}

type ProjectField struct {
	ID       string
	Name     string
	DataType string
	Options  map[string]string
}

type ProjectFieldValue struct {
	Name     string
	OptionID string
	Text     string
}

type ProjectItem struct {
	ID             string
	ContentNodeID  string
	Status         string
	StatusOptionID string
	FieldValues    map[string]ProjectFieldValue
}

type IssueParentInfo struct {
	ID     string
	Number int
	Repo   string
}

type TaskCopy struct {
	Task     Task
	Worktree Worktree
}

type ResolvedTasks struct {
	Tasks      []Task
	ByID       map[string]Task
	BranchByID map[string]string
}

type Counters struct {
	Created       int
	Updated       int
	StatusChanges int
	Imported      int
	ProjectAdded  int
	FieldChanges  int
	SubIssueLinks int
	Failed        int
}

func (cfg Config) Normalized() Config {
	if cfg.ProjectOwnerType == "" {
		cfg.ProjectOwnerType = "user"
	}
	if cfg.MainBranch == "" {
		cfg.MainBranch = "main"
	}
	if cfg.StatusMap == nil {
		cfg.StatusMap = map[string]string{}
	}
	if cfg.Repos == nil {
		cfg.Repos = map[string]string{}
	}
	if cfg.Inbox.Label == "" {
		cfg.Inbox.Label = "inbox"
	}
	if cfg.TimeoutSeconds == 0 {
		cfg.TimeoutSeconds = 60
	}
	if cfg.TaskPrefix == "" {
		cfg.TaskPrefix = "task"
	}
	return cfg
}

func (cfg Config) ConfiguredRepos() []string {
	seen := map[string]bool{}
	var repos []string
	add := func(repo string) {
		repo = strings.TrimSpace(repo)
		if repo == "" || seen[strings.ToLower(repo)] {
			return
		}
		seen[strings.ToLower(repo)] = true
		repos = append(repos, repo)
	}
	for _, repo := range cfg.Repos {
		add(repo)
	}
	add(cfg.DefaultRepo)
	sort.Strings(repos)
	return repos
}

func (cfg Config) RepoAllowed(repo string) bool {
	for _, allowed := range cfg.ConfiguredRepos() {
		if strings.EqualFold(allowed, repo) {
			return true
		}
	}
	return false
}

func (cfg Config) TargetRepo(task Task) string {
	if task.Project != nil && strings.TrimSpace(*task.Project) != "" {
		project := strings.TrimSpace(*task.Project)
		if repo := cfg.Repos[project]; repo != "" {
			return repo
		}
		for key, repo := range cfg.Repos {
			if strings.EqualFold(key, project) {
				return repo
			}
		}
	}
	return cfg.DefaultRepo
}

func (cfg Config) ProjectForRepo(repo string) string {
	keys := make([]string, 0, len(cfg.Repos))
	for project, mapped := range cfg.Repos {
		if strings.EqualFold(mapped, repo) {
			keys = append(keys, project)
		}
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return ""
	}
	return keys[0]
}

func CanonicalTaskID(id string) string { return strings.ToLower(strings.TrimSpace(id)) }
func UpperTaskID(id string) string     { return strings.ToUpper(strings.TrimSpace(id)) }

func IssueTitle(task Task) string { return fmt.Sprintf("%s: %s", CanonicalTaskID(task.ID), task.Title) }
func MarkerFor(id string) string  { return fmt.Sprintf("<!-- backlog:%s -->", CanonicalTaskID(id)) }

func markerRegexp(prefix string) *regexp.Regexp {
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if prefix == "" {
		prefix = "task"
	}
	return regexp.MustCompile(`^<!-- backlog:(` + regexp.QuoteMeta(prefix) + `-[0-9]+(?:\.[0-9]+)*) -->$`)
}

func taskIDRegexp(prefix string) *regexp.Regexp {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "task"
	}
	return regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(prefix) + `-[0-9]+(?:\.[0-9]+)*\b`)
}

func ParseMarker(body string) (string, bool) { return ParseMarkerWithPrefix(body, "task") }

func ParseMarkerWithPrefix(body, prefix string) (string, bool) {
	if body == "" {
		return "", false
	}
	line := body
	if idx := strings.IndexByte(body, '\n'); idx >= 0 {
		line = body[:idx]
	}
	match := markerRegexp(prefix).FindStringSubmatch(strings.TrimRight(line, "\r"))
	if match == nil {
		return "", false
	}
	return match[1], true
}

func LabelsOf(issue Issue) []string {
	labels := make([]string, 0, len(issue.Labels))
	for _, label := range issue.Labels {
		labels = append(labels, label.Name)
	}
	sort.Strings(labels)
	return labels
}

func EqualStringSlices(a, b []string) bool { return EqualStringSlicesFold(a, b, false) }

func EqualStringSlicesFold(a, b []string, fold bool) bool {
	if len(a) != len(b) {
		return false
	}
	aa := append([]string(nil), a...)
	bb := append([]string(nil), b...)
	if fold {
		for i := range aa {
			aa[i] = strings.ToLower(aa[i])
		}
		for i := range bb {
			bb[i] = strings.ToLower(bb[i])
		}
	}
	sort.Strings(aa)
	sort.Strings(bb)
	for i := range aa {
		if aa[i] != bb[i] {
			return false
		}
	}
	return true
}

func DedupSorted(values []string) []string {
	seen := map[string]bool{}
	original := map[string]string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if !seen[key] {
			seen[key] = true
			original[key] = value
		}
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, original[key])
	}
	return out
}
