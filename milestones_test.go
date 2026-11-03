package main

import (
	"context"
	"strings"
	"testing"
)

func TestExecMilestonesIncludesCompleted(t *testing.T) {
	b := ExecBacklog{Runner: &scriptRunner{outputs: map[string][]byte{
		"backlog milestone list --plain --show-completed": []byte(
			"Active milestones:\n  m-1: Next: release (1/3 done)\n\nCompleted milestones:\n  m-0: First: release (3/3 done)\n"),
	}}}
	milestones, err := b.Milestones(context.Background(), "/synthetic")
	if err != nil {
		t.Fatal(err)
	}
	if len(milestones) != 2 || milestones["m-0"] != "First: release" || milestones["m-1"] != "Next: release" {
		t.Fatalf("active/completed titles must retain colons: %v", milestones)
	}
	task := Task{ID: "TASK-1", Title: "Synthetic task", Status: "Done", Milestone: sp("m-0")}
	body := RenderIssueBodyWithOptions(task, RenderOptions{Milestones: milestones})
	if !strings.Contains(body, "Milestone: First: release\n") {
		t.Fatalf("completed milestone not resolved in body: %s", body)
	}
	fields := desiredProjectFieldValues(Config{Fields: FieldConfig{Milestone: "Milestone"}}, task, milestones)
	if fields["milestone"].text != "First: release" {
		t.Fatalf("completed milestone not resolved in project field: %+v", fields)
	}
	task.Milestone = sp("m-99")
	body = RenderIssueBodyWithOptions(task, RenderOptions{Milestones: milestones})
	if !strings.Contains(body, "Milestone: m-99\n") {
		t.Fatalf("unresolved milestone ID must remain explicit, not guessed: %s", body)
	}
}
