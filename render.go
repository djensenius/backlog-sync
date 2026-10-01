package main

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

type RenderOptions struct {
	IssueByTaskID map[string]Issue
	TaskByID      map[string]Task
	Milestones    map[string]string
}

func RenderIssueBody(task Task, issueByTaskID map[string]Issue, taskByID map[string]Task) string {
	return RenderIssueBodyWithOptions(task, RenderOptions{IssueByTaskID: issueByTaskID, TaskByID: taskByID})
}

func RenderIssueBodyWithOptions(task Task, opts RenderOptions) string {
	if opts.IssueByTaskID == nil {
		opts.IssueByTaskID = map[string]Issue{}
	}
	if opts.TaskByID == nil {
		opts.TaskByID = map[string]Task{}
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "%s\n", MarkerFor(task.ID))
	fmt.Fprintf(&b, "%s\n\n", mirrorNotice)
	fmt.Fprintf(&b, "Status: %s\n", task.Status)
	if task.Branch != "" && task.Branch != "main" {
		fmt.Fprintf(&b, "Branch: %s\n", task.Branch)
	}
	fmt.Fprintf(&b, "Project: %s\n", stringPtrOrNone(task.Project))
	fmt.Fprintf(&b, "Milestone: %s\n", milestoneTitle(task.Milestone, opts.Milestones))
	fmt.Fprintf(&b, "Priority: %s\n", stringPtrOrNone(task.Priority))
	fmt.Fprintf(&b, "Labels: %s\n", commaOrNone(task.Labels))
	fmt.Fprintf(&b, "Assignees: %s\n", commaOrNone(task.Assignees))
	fmt.Fprintf(&b, "Parent: %s\n", renderParent(task, opts.IssueByTaskID, opts.TaskByID))
	fmt.Fprintf(&b, "Depends on:\n%s\n", renderDependencyList(task.Dependencies, opts.IssueByTaskID, opts.TaskByID))
	fmt.Fprintf(&b, "Subtasks:\n%s\n\n", renderTaskRefList(task.Subtasks, opts.IssueByTaskID))
	fmt.Fprintf(&b, "## Description\n%s\n\n", blockOrNone(task.Description))
	fmt.Fprintf(&b, "## Acceptance criteria\n")
	if len(task.AcceptanceCriteria) == 0 {
		fmt.Fprintf(&b, "None\n")
	} else {
		criteria := append([]AcceptanceCriterion(nil), task.AcceptanceCriteria...)
		sort.Slice(criteria, func(i, j int) bool { return criteria[i].Index < criteria[j].Index })
		for _, ac := range criteria {
			box := " "
			if ac.Checked {
				box = "x"
			}
			fmt.Fprintf(&b, "- [%s] %s\n", box, ac.Text)
		}
	}
	if strings.TrimSpace(task.ImplementationPlan) != "" {
		fmt.Fprintf(&b, "\n## Implementation plan\n%s\n", strings.TrimSpace(task.ImplementationPlan))
	}
	if strings.TrimSpace(task.ImplementationNotes) != "" {
		fmt.Fprintf(&b, "\n## Implementation notes\n%s\n", strings.TrimSpace(task.ImplementationNotes))
	}
	if task.FinalSummary != nil && strings.TrimSpace(*task.FinalSummary) != "" {
		fmt.Fprintf(&b, "\n## Final summary\n%s\n", strings.TrimSpace(*task.FinalSummary))
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func renderParent(task Task, issueByTaskID map[string]Issue, taskByID map[string]Task) string {
	if task.ParentTaskID == nil || strings.TrimSpace(*task.ParentTaskID) == "" {
		return "None"
	}
	id := CanonicalTaskID(*task.ParentTaskID)
	title := id
	if parent, ok := taskByID[id]; ok {
		title = fmt.Sprintf("%s: %s", CanonicalTaskID(parent.ID), parent.Title)
	}
	if issue, ok := issueByTaskID[id]; ok && issue.HTMLURL != "" {
		return fmt.Sprintf("[%s](%s)", title, issue.HTMLURL)
	}
	return title
}

func renderTaskRefList(refs []TaskRef, issueByTaskID map[string]Issue) string {
	if len(refs) == 0 {
		return "- None"
	}
	refs = append([]TaskRef(nil), refs...)
	sort.Slice(refs, func(i, j int) bool { return CanonicalTaskID(refs[i].ID) < CanonicalTaskID(refs[j].ID) })
	lines := make([]string, 0, len(refs))
	for _, ref := range refs {
		id := CanonicalTaskID(ref.ID)
		text := fmt.Sprintf("%s: %s", id, ref.Title)
		if issue, ok := issueByTaskID[id]; ok && issue.HTMLURL != "" {
			text = fmt.Sprintf("[%s](%s)", text, issue.HTMLURL)
		}
		lines = append(lines, "- "+text)
	}
	return strings.Join(lines, "\n")
}

func renderDependencyList(ids []string, issueByTaskID map[string]Issue, taskByID map[string]Task) string {
	if len(ids) == 0 {
		return "- None"
	}
	ids = append([]string(nil), ids...)
	sort.Slice(ids, func(i, j int) bool { return CanonicalTaskID(ids[i]) < CanonicalTaskID(ids[j]) })
	lines := make([]string, 0, len(ids))
	for _, raw := range ids {
		id := CanonicalTaskID(raw)
		text := id
		if task, ok := taskByID[id]; ok {
			text = fmt.Sprintf("%s: %s", CanonicalTaskID(task.ID), task.Title)
		}
		if issue, ok := issueByTaskID[id]; ok && issue.HTMLURL != "" {
			text = fmt.Sprintf("[%s](%s)", text, issue.HTMLURL)
		}
		lines = append(lines, "- "+text)
	}
	return strings.Join(lines, "\n")
}

func commaOrNone(values []string) string {
	if len(values) == 0 {
		return "None"
	}
	values = append([]string(nil), values...)
	sort.Strings(values)
	return strings.Join(values, ", ")
}

func stringPtrOrNone(value *string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return "None"
	}
	return *value
}

func milestoneTitle(value *string, milestones map[string]string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return "None"
	}
	if title := milestones[*value]; title != "" {
		return title
	}
	return *value
}

func blockOrNone(value string) string {
	if strings.TrimSpace(value) == "" {
		return "None"
	}
	return strings.TrimSpace(value)
}
