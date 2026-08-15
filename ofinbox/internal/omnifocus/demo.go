package omnifocus

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DemoClient is an in-memory Client with sample data, for trying the TUI
// without OmniFocus (or on non-macOS machines) and for tests.
type DemoClient struct {
	mu       sync.Mutex
	tasks    []Task
	projects []Project
	tags     []Tag
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
		},
		projects: []Project{
			{ID: "p1", Name: "Home maintenance", Folder: "Personal", Status: "active"},
			{ID: "p2", Name: "Health", Folder: "Personal", Status: "active"},
			{ID: "p3", Name: "Travel prep", Folder: "Personal", Status: "active"},
			{ID: "p4", Name: "Experiments repo", Folder: "Work", Status: "active"},
			{ID: "p5", Name: "Reading list", Folder: "", Status: "active"},
			{ID: "p6", Name: "Kitchen remodel", Folder: "Personal", Status: "on hold"},
		},
		tags: []Tag{
			{ID: "g1", Name: "errand"},
			{ID: "g2", Name: "email"},
			{ID: "g3", Name: "deep work"},
			{ID: "g4", Name: "waiting"},
			{ID: "g5", Name: "phone"},
		},
	}
}

func (c *DemoClient) InboxTasks(ctx context.Context) ([]Task, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Task, len(c.tasks))
	copy(out, c.tasks)
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

func (c *DemoClient) find(taskID string) (int, error) {
	for i, t := range c.tasks {
		if t.ID == taskID {
			return i, nil
		}
	}
	return -1, fmt.Errorf("task not found: %s", taskID)
}

func (c *DemoClient) remove(taskID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	i, err := c.find(taskID)
	if err != nil {
		return err
	}
	c.tasks = append(c.tasks[:i], c.tasks[i+1:]...)
	return nil
}

func (c *DemoClient) Complete(ctx context.Context, taskID string) error {
	return c.remove(taskID)
}

func (c *DemoClient) Drop(ctx context.Context, taskID string) error {
	return c.remove(taskID)
}

func (c *DemoClient) MoveToProject(ctx context.Context, taskID, projectID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range c.projects {
		if p.ID == projectID {
			i, err := c.find(taskID)
			if err != nil {
				return err
			}
			c.tasks = append(c.tasks[:i], c.tasks[i+1:]...)
			return nil
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
