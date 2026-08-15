package omnifocus

import (
	"context"
	"testing"
	"time"
)

func TestDemoClientLifecycle(t *testing.T) {
	ctx := context.Background()
	c := NewDemoClient()

	tasks, err := c.InboxTasks(ctx)
	if err != nil || len(tasks) == 0 {
		t.Fatalf("InboxTasks: %v (%d tasks)", err, len(tasks))
	}
	initial := len(tasks)
	first := tasks[0]

	if err := c.Complete(ctx, first.ID); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	tasks, _ = c.InboxTasks(ctx)
	if len(tasks) != initial-1 {
		t.Fatalf("after Complete: %d tasks, want %d", len(tasks), initial-1)
	}

	projects, err := c.Projects(ctx)
	if err != nil || len(projects) == 0 {
		t.Fatalf("Projects: %v", err)
	}
	if err := c.MoveToProject(ctx, tasks[0].ID, projects[0].ID); err != nil {
		t.Fatalf("MoveToProject: %v", err)
	}
	tasks, _ = c.InboxTasks(ctx)
	if len(tasks) != initial-2 {
		t.Fatalf("after MoveToProject: %d tasks, want %d", len(tasks), initial-2)
	}

	tags, err := c.Tags(ctx)
	if err != nil || len(tags) == 0 {
		t.Fatalf("Tags: %v", err)
	}
	target := tasks[0]
	if err := c.AddTag(ctx, target.ID, tags[0].ID); err != nil {
		t.Fatalf("AddTag: %v", err)
	}
	// Adding the same tag twice stays idempotent.
	if err := c.AddTag(ctx, target.ID, tags[0].ID); err != nil {
		t.Fatalf("AddTag (repeat): %v", err)
	}
	tasks, _ = c.InboxTasks(ctx)
	count := 0
	for _, name := range tasks[0].Tags {
		if name == tags[0].Name {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("tag %q appears %d times, want 1", tags[0].Name, count)
	}

	if err := c.Rename(ctx, target.ID, "renamed"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if err := c.SetFlagged(ctx, target.ID, true); err != nil {
		t.Fatalf("SetFlagged: %v", err)
	}
	due := time.Now().Add(24 * time.Hour)
	if err := c.SetDueDate(ctx, target.ID, &due); err != nil {
		t.Fatalf("SetDueDate: %v", err)
	}
	if err := c.SetDueDate(ctx, target.ID, nil); err != nil {
		t.Fatalf("SetDueDate (clear): %v", err)
	}

	tasks, _ = c.InboxTasks(ctx)
	got := tasks[0]
	if got.Name != "renamed" || !got.Flagged || got.DueDate != nil {
		t.Fatalf("task state after edits: %+v", got)
	}

	if err := c.Complete(ctx, "no-such-id"); err == nil {
		t.Fatal("Complete with bad ID: expected error")
	}
}
