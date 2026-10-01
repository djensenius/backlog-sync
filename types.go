package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

const mirrorNotice = "Mirrored one-way from Backlog.md — edits here are overwritten. Change it with the backlog CLI."

var markerRE = regexp.MustCompile(`^<!-- backlog:(task-[0-9]+(?:\.[0-9]+)*) -->$`)

type Config struct {
	Root          string
	Repo          string
	ProjectOwner  string
	ProjectNumber int
	DryRun        bool
	NoInbox       bool
	Verbose       bool
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
	Total    int           `json:"total"`
	NextSkip *int          `json:"nextSkip"`
}

type TaskViewResponse struct {
	Task Task `json:"task"`
}

type Task struct {
	ID                 string                `json:"id"`
	Title              string                `json:"title"`
	Status             string                `json:"status"`
	Labels             []string              `json:"labels"`
	Milestone          *string               `json:"milestone"`
	ParentTaskID       *string               `json:"parentTaskId"`
	Assignees          []string              `json:"assignees"`
	CreatedAt          *time.Time            `json:"createdAt"`
	UpdatedAt          *time.Time            `json:"updatedAt"`
	Description        string                `json:"description"`
	Dependencies       []string              `json:"dependencies"`
	Subtasks           []TaskRef             `json:"subtasks"`
	AcceptanceCriteria []AcceptanceCriterion `json:"acceptanceCriteria"`
	FinalSummary       *string               `json:"finalSummary"`
	Branch             string                `json:"-"`
	WorktreePath       string                `json:"-"`
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
	NodeID      string       `json:"node_id"`
	HTMLURL     string       `json:"html_url"`
	Title       string       `json:"title"`
	Body        string       `json:"body"`
	State       string       `json:"state"`
	StateReason string       `json:"state_reason"`
	Labels      []IssueLabel `json:"labels"`
	PullRequest *struct{}    `json:"pull_request"`
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
}

type ProjectItem struct {
	ID             string
	ContentNodeID  string
	Status         string
	StatusOptionID string
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
}

func CanonicalTaskID(id string) string {
	return strings.ToLower(id)
}

func UpperTaskID(id string) string {
	return strings.ToUpper(id)
}

func IssueTitle(task Task) string {
	return fmt.Sprintf("%s: %s", CanonicalTaskID(task.ID), task.Title)
}

func MarkerFor(id string) string {
	return fmt.Sprintf("<!-- backlog:%s -->", CanonicalTaskID(id))
}

func ParseMarker(body string) (string, bool) {
	if body == "" {
		return "", false
	}
	line := body
	if idx := strings.IndexByte(body, '\n'); idx >= 0 {
		line = body[:idx]
	}
	match := markerRE.FindStringSubmatch(strings.TrimRight(line, "\r"))
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

func EqualStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aa := append([]string(nil), a...)
	bb := append([]string(nil), b...)
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
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
