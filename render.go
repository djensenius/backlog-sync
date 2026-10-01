package main

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

const (
	githubIssueBodyCharacterLimit = 65536
	// GitHub documents its issue body limit in characters. Count Go runes (Unicode
	// scalar values) rather than bytes so multi-byte UTF-8 text receives the same
	// budget as single-byte text, and leave margin below GitHub's hard limit.
	issueBodyCharacterBudget = 60000
)

type RenderOptions struct {
	IssueByTaskID map[string]Issue
	TaskByID      map[string]Task
	Milestones    map[string]string
	MainBranch    string
}

type issueBodyDocument struct {
	TaskID              string
	Header              string
	Description         string
	AcceptanceCriteria  string
	ImplementationPlan  *string
	ImplementationNotes *string
	FinalSummary        *string
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
	if opts.MainBranch == "" {
		opts.MainBranch = "main"
	}
	doc := buildIssueBodyDocument(task, opts)
	return renderIssueBodyWithinBudget(doc, issueBodyCharacterBudget)
}

func buildIssueBodyDocument(task Task, opts RenderOptions) issueBodyDocument {
	var header bytes.Buffer
	fmt.Fprintf(&header, "%s\n", MarkerFor(task.ID))
	fmt.Fprintf(&header, "%s\n\n", mirrorNotice)
	fmt.Fprintf(&header, "Status: %s\n", task.Status)
	if task.Branch != "" && task.Branch != opts.MainBranch {
		fmt.Fprintf(&header, "Branch: %s\n", task.Branch)
	}
	fmt.Fprintf(&header, "Project: %s\n", stringPtrOrNone(task.Project))
	fmt.Fprintf(&header, "Milestone: %s\n", milestoneTitle(task.Milestone, opts.Milestones))
	fmt.Fprintf(&header, "Priority: %s\n", stringPtrOrNone(task.Priority))
	fmt.Fprintf(&header, "Labels: %s\n", commaOrNone(task.Labels))
	fmt.Fprintf(&header, "Assignees: %s\n", commaOrNone(task.Assignees))
	fmt.Fprintf(&header, "Parent: %s\n", renderParent(task, opts.IssueByTaskID, opts.TaskByID))
	fmt.Fprintf(&header, "Depends on:\n%s\n", renderDependencyList(task.Dependencies, opts.IssueByTaskID, opts.TaskByID))
	fmt.Fprintf(&header, "Subtasks:\n%s\n\n", renderTaskRefList(task.Subtasks, opts.IssueByTaskID))

	var criteria bytes.Buffer
	fmt.Fprintf(&criteria, "## Acceptance criteria\n")
	if len(task.AcceptanceCriteria) == 0 {
		fmt.Fprintf(&criteria, "None\n")
	} else {
		items := append([]AcceptanceCriterion(nil), task.AcceptanceCriteria...)
		sort.Slice(items, func(i, j int) bool { return items[i].Index < items[j].Index })
		for _, ac := range items {
			box := " "
			if ac.Checked {
				box = "x"
			}
			fmt.Fprintf(&criteria, "- [%s] %s\n", box, ac.Text)
		}
	}

	doc := issueBodyDocument{
		TaskID:             CanonicalTaskID(task.ID),
		Header:             header.String(),
		Description:        blockOrNone(task.Description),
		AcceptanceCriteria: criteria.String(),
	}
	if plan := strings.TrimSpace(task.ImplementationPlan); plan != "" {
		doc.ImplementationPlan = &plan
	}
	if notes := strings.TrimSpace(task.ImplementationNotes); notes != "" {
		doc.ImplementationNotes = &notes
	}
	if task.FinalSummary != nil {
		if summary := strings.TrimSpace(*task.FinalSummary); summary != "" {
			doc.FinalSummary = &summary
		}
	}
	return doc
}

func renderIssueBodyWithinBudget(doc issueBodyDocument, budget int) string {
	if budget <= 0 {
		return doc.String()
	}
	if countCharacters(doc.String()) <= budget {
		return doc.String()
	}
	sections := []*string{doc.ImplementationNotes, doc.ImplementationPlan, &doc.Description, doc.FinalSummary}
	for _, section := range sections {
		if section == nil || *section == "" {
			continue
		}
		if countCharacters(doc.String()) <= budget {
			break
		}
		truncateIssueSectionToFit(&doc, section, budget)
	}
	return doc.String()
}

func (d issueBodyDocument) String() string {
	var b bytes.Buffer
	b.WriteString(d.Header)
	fmt.Fprintf(&b, "## Description\n%s\n\n", d.Description)
	b.WriteString(d.AcceptanceCriteria)
	if d.ImplementationPlan != nil && strings.TrimSpace(*d.ImplementationPlan) != "" {
		fmt.Fprintf(&b, "\n## Implementation plan\n%s\n", strings.TrimSpace(*d.ImplementationPlan))
	}
	if d.ImplementationNotes != nil && strings.TrimSpace(*d.ImplementationNotes) != "" {
		fmt.Fprintf(&b, "\n## Implementation notes\n%s\n", strings.TrimSpace(*d.ImplementationNotes))
	}
	if d.FinalSummary != nil && strings.TrimSpace(*d.FinalSummary) != "" {
		fmt.Fprintf(&b, "\n## Final summary\n%s\n", strings.TrimSpace(*d.FinalSummary))
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func truncateIssueSectionToFit(doc *issueBodyDocument, section *string, budget int) {
	original := *section
	originalRunes := []rune(original)
	if len(originalRunes) == 0 {
		return
	}
	best := -1
	low, high := 0, len(originalRunes)
	for low <= high {
		mid := low + (high-low)/2
		candidate := truncatedIssueSectionContent(original, mid, doc.TaskID)
		*section = candidate
		if countCharacters(doc.String()) <= budget {
			best = mid
			low = mid + 1
		} else {
			high = mid - 1
		}
	}
	if best < 0 {
		*section = original
		return
	}
	retained := best
	for i := best; i > 0; i-- {
		if originalRunes[i-1] == '\n' {
			retained = i - 1
			break
		}
	}
	*section = truncatedIssueSectionContent(original, retained, doc.TaskID)
}

func truncatedIssueSectionContent(original string, retained int, taskID string) string {
	runes := []rune(original)
	if retained > len(runes) {
		retained = len(runes)
	}
	if retained < 0 {
		retained = 0
	}
	prefix := strings.TrimRight(string(runes[:retained]), "\n")
	omitted := len(runes) - countCharacters(prefix)
	marker := fmt.Sprintf("… truncated (%d characters omitted); see `backlog task view %s --plain`", omitted, CanonicalTaskID(taskID))
	if strings.TrimSpace(prefix) == "" {
		return marker
	}
	return prefix + "\n" + marker
}

func countCharacters(value string) int { return len([]rune(value)) }

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
