// Package omnifocus defines the data model and client interface for talking
// to OmniFocus, plus two implementations: one backed by osascript/JXA (macOS)
// and an in-memory demo backend for development anywhere.
package omnifocus

import (
	"context"
	"time"
)

// Task is an OmniFocus task (typically an inbox item).
type Task struct {
	ID        string
	Name      string
	Note      string
	Flagged   bool
	DeferDate *time.Time
	DueDate   *time.Time
	Tags      []string
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

// Client is the operation set the TUI needs. All mutations act on task IDs so
// the UI never holds live references to OmniFocus objects.
type Client interface {
	InboxTasks(ctx context.Context) ([]Task, error)
	Projects(ctx context.Context) ([]Project, error)
	Tags(ctx context.Context) ([]Tag, error)

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
