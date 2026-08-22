package omnifocus

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

//go:embed scripts/*.js
var scripts embed.FS

// OsascriptClient talks to OmniFocus on macOS by running embedded JXA
// (JavaScript for Automation) scripts through /usr/bin/osascript. No
// third-party CLI is required; the first run prompts for an Automation
// permission grant (System Settings → Privacy & Security → Automation).
type OsascriptClient struct{}

func NewOsascriptClient() *OsascriptClient { return &OsascriptClient{} }

func (c *OsascriptClient) run(ctx context.Context, script string, args ...string) (string, error) {
	src, err := scripts.ReadFile("scripts/" + script)
	if err != nil {
		return "", fmt.Errorf("embedded script %s: %w", script, err)
	}
	cmdArgs := append([]string{"-l", "JavaScript", "-e", string(src)}, args...)
	cmd := exec.CommandContext(ctx, "osascript", cmdArgs...)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			return "", fmt.Errorf("osascript: %s", firstLine(string(ee.Stderr)))
		}
		return "", fmt.Errorf("osascript: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

type taskJSON struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Note       string   `json:"note"`
	Flagged    bool     `json:"flagged"`
	DeferDate  *string  `json:"deferDate"`
	DueDate    *string  `json:"dueDate"`
	Tags       []string `json:"tags"`
	ChildCount int      `json:"childCount"`
	ParentID   string   `json:"parentID"`
}

func parseISO(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil
	}
	local := t.Local()
	return &local
}

func parseTasks(out string) ([]Task, error) {
	var raw []taskJSON
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("parsing task JSON: %w", err)
	}
	tasks := make([]Task, 0, len(raw))
	for _, r := range raw {
		tasks = append(tasks, Task{
			ID:         r.ID,
			Name:       r.Name,
			Note:       r.Note,
			Flagged:    r.Flagged,
			DeferDate:  parseISO(r.DeferDate),
			DueDate:    parseISO(r.DueDate),
			Tags:       r.Tags,
			ChildCount: r.ChildCount,
			ParentID:   r.ParentID,
		})
	}
	return tasks, nil
}

func (c *OsascriptClient) InboxTasks(ctx context.Context) ([]Task, error) {
	out, err := c.run(ctx, "inbox.js")
	if err != nil {
		return nil, err
	}
	return parseTasks(out)
}

func (c *OsascriptClient) Children(ctx context.Context, taskID string) ([]Task, error) {
	out, err := c.run(ctx, "children.js", taskID)
	if err != nil {
		return nil, err
	}
	return parseTasks(out)
}

func (c *OsascriptClient) Projects(ctx context.Context) ([]Project, error) {
	out, err := c.run(ctx, "projects.js")
	if err != nil {
		return nil, err
	}
	var projects []Project
	type projJSON struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Folder string `json:"folder"`
		Status string `json:"status"`
	}
	var raw []projJSON
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("parsing projects JSON: %w", err)
	}
	for _, r := range raw {
		projects = append(projects, Project(r))
	}
	return projects, nil
}

func (c *OsascriptClient) Tags(ctx context.Context) ([]Tag, error) {
	out, err := c.run(ctx, "tags.js")
	if err != nil {
		return nil, err
	}
	var tags []Tag
	if err := json.Unmarshal([]byte(out), &tags); err != nil {
		return nil, fmt.Errorf("parsing tags JSON: %w", err)
	}
	return tags, nil
}

func (c *OsascriptClient) History(ctx context.Context) ([]HistoryEntry, error) {
	out, err := c.run(ctx, "history.js")
	if err != nil {
		return nil, err
	}
	type histJSON struct {
		Name      string   `json:"name"`
		ProjectID string   `json:"projectID"`
		Tags      []string `json:"tags"`
	}
	var raw []histJSON
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("parsing history JSON: %w", err)
	}
	entries := make([]HistoryEntry, 0, len(raw))
	for _, r := range raw {
		entries = append(entries, HistoryEntry{Name: r.Name, ProjectID: r.ProjectID, TagIDs: r.Tags})
	}
	return entries, nil
}

func (c *OsascriptClient) create(ctx context.Context, kind, name string) (string, string, error) {
	out, err := c.run(ctx, "create.js", kind, name)
	if err != nil {
		return "", "", err
	}
	var made struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(out), &made); err != nil {
		return "", "", fmt.Errorf("parsing create JSON: %w", err)
	}
	return made.ID, made.Name, nil
}

func (c *OsascriptClient) CreateProject(ctx context.Context, name string) (Project, error) {
	id, madeName, err := c.create(ctx, "project", name)
	if err != nil {
		return Project{}, err
	}
	return Project{ID: id, Name: madeName, Status: "active"}, nil
}

func (c *OsascriptClient) CreateTag(ctx context.Context, name string) (Tag, error) {
	id, madeName, err := c.create(ctx, "tag", name)
	if err != nil {
		return Tag{}, err
	}
	return Tag{ID: id, Name: madeName}, nil
}

func (c *OsascriptClient) action(ctx context.Context, args ...string) error {
	_, err := c.run(ctx, "task_action.js", args...)
	return err
}

func (c *OsascriptClient) Complete(ctx context.Context, taskID string) error {
	return c.action(ctx, "complete", taskID)
}

func (c *OsascriptClient) Drop(ctx context.Context, taskID string) error {
	return c.action(ctx, "drop", taskID)
}

func (c *OsascriptClient) MoveToProject(ctx context.Context, taskID, projectID string) error {
	return c.action(ctx, "move", taskID, projectID)
}

func (c *OsascriptClient) AddTag(ctx context.Context, taskID, tagID string) error {
	return c.action(ctx, "tag", taskID, tagID)
}

func (c *OsascriptClient) SetFlagged(ctx context.Context, taskID string, flagged bool) error {
	return c.action(ctx, "flag", taskID, strconv.FormatBool(flagged))
}

func (c *OsascriptClient) Rename(ctx context.Context, taskID, name string) error {
	return c.action(ctx, "rename", taskID, name)
}

func (c *OsascriptClient) SetDeferDate(ctx context.Context, taskID string, t *time.Time) error {
	return c.action(ctx, "defer", taskID, isoOrEmpty(t))
}

func (c *OsascriptClient) SetDueDate(ctx context.Context, taskID string, t *time.Time) error {
	return c.action(ctx, "due", taskID, isoOrEmpty(t))
}

func isoOrEmpty(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

var _ Client = (*OsascriptClient)(nil)
