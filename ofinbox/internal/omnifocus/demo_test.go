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

func TestLinkURL(t *testing.T) {
	cases := []struct {
		name string
		task Task
		url  string
	}{
		{"untitled with URL note", Task{Note: "https://example.com/a"}, "https://example.com/a"},
		{"URL-only title", Task{Name: "https://example.com/b"}, "https://example.com/b"},
		{"title with URL-only note", Task{Name: "Read later", Note: "https://example.com/c"}, "https://example.com/c"},
		{"URL note with surrounding whitespace", Task{Note: "\n  https://example.com/d \n"}, "https://example.com/d"},
		{"http scheme", Task{Note: "http://example.com/e"}, "http://example.com/e"},
		{"plain task", Task{Name: "Buy filter", Note: "16x25x1"}, ""},
		{"URL amid prose in note", Task{Name: "Read later", Note: "see https://example.com/f for details"}, ""},
		{"multi-line note starting with URL", Task{Note: "https://example.com/g\nplus notes"}, ""},
		{"non-http scheme", Task{Note: "ftp://example.com/h"}, ""},
		{"empty task", Task{}, ""},
	}
	for _, c := range cases {
		url, ok := c.task.LinkURL()
		if want := c.url != ""; ok != want || url != c.url {
			t.Errorf("%s: LinkURL() = %q, %v; want %q, %v", c.name, url, ok, c.url, want)
		}
	}
}

func TestDemoClientChildren(t *testing.T) {
	ctx := context.Background()
	c := NewDemoClient()

	tasks, _ := c.InboxTasks(ctx)
	var group Task
	for _, task := range tasks {
		if task.ParentID != "" {
			t.Fatalf("InboxTasks returned a child: %+v", task)
		}
		if task.ChildCount > 0 {
			group = task
		}
	}
	if group.ID == "" {
		t.Fatal("demo data has no action group")
	}

	kids, err := c.Children(ctx, group.ID)
	if err != nil || len(kids) != group.ChildCount {
		t.Fatalf("Children: %v (%d kids, want %d)", err, len(kids), group.ChildCount)
	}
	var nested Task
	for _, k := range kids {
		if k.ParentID != group.ID {
			t.Fatalf("child %s has ParentID %q, want %q", k.ID, k.ParentID, group.ID)
		}
		if k.ChildCount > 0 {
			nested = k
		}
	}
	if nested.ID == "" {
		t.Fatal("demo data has no nested action group")
	}

	// Completing the group cascades to all descendants.
	if err := c.Complete(ctx, group.ID); err != nil {
		t.Fatalf("Complete group: %v", err)
	}
	if _, err := c.Children(ctx, nested.ID); err == nil {
		t.Fatal("nested child survived cascade")
	}

	if _, err := c.Children(ctx, "no-such-id"); err == nil {
		t.Fatal("Children with bad ID: expected error")
	}
}
