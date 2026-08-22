package omnifocus

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DemoClient is an in-memory Client with sample data, for trying the TUI
// without OmniFocus (or on non-macOS machines) and for tests.
//
// tasks holds top-level items and children in one flat slice, linked by
// ParentID; ChildCount is derived, never stored.
type DemoClient struct {
	mu       sync.Mutex
	tasks    []Task
	projects []Project
	tags     []Tag
	history  []HistoryEntry
}

func NewDemoClient() *DemoClient {
	now := time.Now()
	tomorrow := now.AddDate(0, 0, 1)
	return &DemoClient{
		tasks: []Task{
			{ID: "t1", Name: "Book dentist appointment", Note: "Dr. Alvarez said to come back in 6 months."},
			{ID: "t2", Name: "Reply to Sam about the offsite", Flagged: true, Tags: []string{"email"}},
			{ID: "t3", Name: "Renew passport", Note: "Expires in November. Need new photos first.\nForm DS-82 can be filed by mail.", DueDate: &tomorrow},
			{ID: "t4", Name: "Fix flaky CI job on experiments repo"},
			{ID: "t5", Name: "Read \"A Philosophy of Software Design\""},
			{ID: "t6", Name: "Buy replacement furnace filter", Note: "16x25x1 MERV 11"},
			{ID: "t7", Name: "Plan Tahoe ski weekend", Note: "Captured from email thread with Sam."},
			{ID: "t7a", ParentID: "t7", Name: "Check cabin availability"},
			{ID: "t7b", ParentID: "t7", Name: "Compare lift ticket prices", Tags: []string{"errand"}},
			{ID: "t7c", ParentID: "t7", Name: "Sort out rental gear"},
			{ID: "t7c1", ParentID: "t7c", Name: "Find ski boots in garage"},
			{ID: "t8", Name: "", Note: "https://example.com/great-article"},
			{ID: "t9", Name: "https://go.dev/blog/loopvar-preview"},
			{ID: "t10", Name: "Retirement cash flow calculator", Note: "https://example.com/retirement-cash-flow"},
		},
		projects: []Project{
			{ID: "p1", Name: "Home maintenance", Folder: "Personal", Status: "active"},
			{ID: "p2", Name: "Health", Folder: "Personal", Status: "active"},
			{ID: "p3", Name: "Travel prep", Folder: "Personal", Status: "active"},
			{ID: "p4", Name: "Experiments repo", Folder: "Work", Status: "active"},
			{ID: "p5", Name: "Reading list", Folder: "", Status: "active"},
			{ID: "p6", Name: "Kitchen remodel", Folder: "Personal", Status: "on hold"},
			{ID: "p7", Name: "Links to Review", Folder: "Personal", Status: "active"},
		},
		tags: []Tag{
			{ID: "g1", Name: "errand"},
			{ID: "g2", Name: "email"},
			{ID: "g3", Name: "deep work"},
			{ID: "g4", Name: "waiting"},
			{ID: "g5", Name: "phone"},
			{ID: "g6", Name: "NoAction"},
		},
		history: []HistoryEntry{
			{Name: "Schedule annual physical", ProjectID: "p2", TagIDs: []string{"g5"}},
			{Name: "Dentist cleaning", ProjectID: "p2", TagIDs: []string{"g5"}},
			{Name: "Refill allergy prescription", ProjectID: "p2", TagIDs: []string{"g1"}},
			{Name: "Replace furnace filter", ProjectID: "p1", TagIDs: []string{"g1"}},
			{Name: "Clean the gutters", ProjectID: "p1"},
			{Name: "Fix CI pipeline timeout", ProjectID: "p4", TagIDs: []string{"g3"}},
			{Name: "Upgrade Go toolchain on experiments repo", ProjectID: "p4"},
			{Name: "Renew Global Entry", ProjectID: "p3", TagIDs: []string{"g5"}},
			{Name: "Get passport photos taken", ProjectID: "p3", TagIDs: []string{"g1"}},
			{Name: "Read \"Designing Data-Intensive Applications\"", ProjectID: "p5", TagIDs: []string{"g3"}},
			{Name: "Reply to landlord about lease renewal", ProjectID: "p1", TagIDs: []string{"g2"}},
		},
	}
}

// childCount counts direct children. Callers must hold c.mu.
func (c *DemoClient) childCount(id string) int {
	n := 0
	for _, t := range c.tasks {
		if t.ParentID == id {
			n++
		}
	}
	return n
}

func (c *DemoClient) InboxTasks(ctx context.Context) ([]Task, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []Task
	for _, t := range c.tasks {
		if t.ParentID == "" {
			t.ChildCount = c.childCount(t.ID)
			out = append(out, t)
		}
	}
	return out, nil
}

func (c *DemoClient) Children(ctx context.Context, taskID string) ([]Task, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := c.find(taskID); err != nil {
		return nil, err
	}
	var out []Task
	for _, t := range c.tasks {
		if t.ParentID == taskID {
			t.ChildCount = c.childCount(t.ID)
			out = append(out, t)
		}
	}
	return out, nil
}

func (c *DemoClient) Projects(ctx context.Context) ([]Project, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Project, len(c.projects))
	copy(out, c.projects)
	return out, nil
}

func (c *DemoClient) Tags(ctx context.Context) ([]Tag, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Tag, len(c.tags))
	copy(out, c.tags)
	return out, nil
}

func (c *DemoClient) History(ctx context.Context) ([]HistoryEntry, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]HistoryEntry, len(c.history))
	copy(out, c.history)
	return out, nil
}

func (c *DemoClient) CreateProject(ctx context.Context, name string) (Project, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	p := Project{ID: fmt.Sprintf("p-new-%d", len(c.projects)+1), Name: name, Status: "active"}
	c.projects = append(c.projects, p)
	return p, nil
}

func (c *DemoClient) CreateTag(ctx context.Context, name string) (Tag, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	g := Tag{ID: fmt.Sprintf("g-new-%d", len(c.tags)+1), Name: name}
	c.tags = append(c.tags, g)
	return g, nil
}

func (c *DemoClient) find(taskID string) (int, error) {
	for i, t := range c.tasks {
		if t.ID == taskID {
			return i, nil
		}
	}
	return -1, fmt.Errorf("task not found: %s", taskID)
}

// remove deletes a task and all its descendants, mirroring OmniFocus's
// cascade on complete/drop/move of an action group. Callers must hold c.mu.
func (c *DemoClient) remove(taskID string) error {
	if _, err := c.find(taskID); err != nil {
		return err
	}
	doomed := map[string]bool{taskID: true}
	for grew := true; grew; {
		grew = false
		for _, t := range c.tasks {
			if t.ParentID != "" && doomed[t.ParentID] && !doomed[t.ID] {
				doomed[t.ID] = true
				grew = true
			}
		}
	}
	kept := c.tasks[:0]
	for _, t := range c.tasks {
		if !doomed[t.ID] {
			kept = append(kept, t)
		}
	}
	c.tasks = kept
	return nil
}

func (c *DemoClient) Complete(ctx context.Context, taskID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.remove(taskID)
}

func (c *DemoClient) Drop(ctx context.Context, taskID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.remove(taskID)
}

func (c *DemoClient) MoveToProject(ctx context.Context, taskID, projectID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range c.projects {
		if p.ID == projectID {
			return c.remove(taskID)
		}
	}
	return fmt.Errorf("project not found: %s", projectID)
}

func (c *DemoClient) AddTag(ctx context.Context, taskID, tagID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	i, err := c.find(taskID)
	if err != nil {
		return err
	}
	for _, g := range c.tags {
		if g.ID == tagID {
			for _, existing := range c.tasks[i].Tags {
				if existing == g.Name {
					return nil
				}
			}
			c.tasks[i].Tags = append(c.tasks[i].Tags, g.Name)
			return nil
		}
	}
	return fmt.Errorf("tag not found: %s", tagID)
}

func (c *DemoClient) SetFlagged(ctx context.Context, taskID string, flagged bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	i, err := c.find(taskID)
	if err != nil {
		return err
	}
	c.tasks[i].Flagged = flagged
	return nil
}

func (c *DemoClient) Rename(ctx context.Context, taskID, name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	i, err := c.find(taskID)
	if err != nil {
		return err
	}
	c.tasks[i].Name = name
	return nil
}

func (c *DemoClient) SetDeferDate(ctx context.Context, taskID string, t *time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	i, err := c.find(taskID)
	if err != nil {
		return err
	}
	c.tasks[i].DeferDate = t
	return nil
}

func (c *DemoClient) SetDueDate(ctx context.Context, taskID string, t *time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	i, err := c.find(taskID)
	if err != nil {
		return err
	}
	c.tasks[i].DueDate = t
	return nil
}

var _ Client = (*DemoClient)(nil)
