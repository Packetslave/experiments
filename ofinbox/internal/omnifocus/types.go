// Package omnifocus defines the data model and client interface for talking
// to OmniFocus, plus two implementations: one backed by osascript/JXA (macOS)
// and an in-memory demo backend for development anywhere.
package omnifocus

import (
	"context"
	"strings"
	"time"
)

// Task is an OmniFocus task (typically an inbox item).
type Task struct {
	ID         string
	Name       string
	Note       string
	Flagged    bool
	DeferDate  *time.Time
	DueDate    *time.Time
	Tags       []string
	ChildCount int    // direct subtasks; > 0 means this is an action group
	ParentID   string // containing task's ID for fetched children, "" for top-level inbox items
}

// LinkURL reports whether the task is a captured link and returns its URL.
// A task is a link when its name or its note, trimmed, is exactly one URL:
// a URL-only note counts with or without a title, and a URL-only title
// counts regardless of the note.
func (t Task) LinkURL() (string, bool) {
	if u, ok := onlyURL(t.Name); ok {
		return u, true
	}
	return onlyURL(t.Note)
}

func onlyURL(s string) (string, bool) {
	s = strings.TrimSpace(s)
	lower := strings.ToLower(s)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return "", false
	}
	if strings.ContainsAny(s, " \t\n\r") {
		return "", false
	}
	return s, true
}

// Project is a filing destination.
type Project struct {
	ID     string
	Name   string
	Folder string // containing folder name, "" if top-level
	Status string // e.g. "active", "on hold"
}

// Tag is an OmniFocus tag.
type Tag struct {
	ID   string
	Name string
}

// HistoryEntry is one past filing decision: a task already living in a
// project (remaining or completed, not dropped), used to score suggestions.
type HistoryEntry struct {
	Name      string
	ProjectID string
	TagIDs    []string
}

// Client is the operation set the TUI needs. All mutations act on task IDs so
// the UI never holds live references to OmniFocus objects.
type Client interface {
	InboxTasks(ctx context.Context) ([]Task, error)
	// Children returns the direct subtasks of an action group, with ParentID set.
	Children(ctx context.Context, taskID string) ([]Task, error)
	Projects(ctx context.Context) ([]Project, error)
	Tags(ctx context.Context) ([]Tag, error)
	// History returns past filing decisions for suggestion scoring.
	History(ctx context.Context) ([]HistoryEntry, error)

	// CreateProject / CreateTag create a new top-level project (active) or tag
	// and return it with its assigned ID.
	CreateProject(ctx context.Context, name string) (Project, error)
	CreateTag(ctx context.Context, name string) (Tag, error)

	Complete(ctx context.Context, taskID string) error
	Drop(ctx context.Context, taskID string) error
	MoveToProject(ctx context.Context, taskID, projectID string) error
	AddTag(ctx context.Context, taskID, tagID string) error
	SetFlagged(ctx context.Context, taskID string, flagged bool) error
	Rename(ctx context.Context, taskID, name string) error
	// SetDeferDate / SetDueDate clear the date when t is nil.
	SetDeferDate(ctx context.Context, taskID string, t *time.Time) error
	SetDueDate(ctx context.Context, taskID string, t *time.Time) error
}
