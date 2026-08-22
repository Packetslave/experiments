package suggest

// Temporary live smoke test against real OmniFocus data. Run with:
//   OFINBOX_LIVE=1 go test ./internal/suggest/ -run TestLive -v

import (
	"context"
	"os"
	"testing"

	"github.com/packetslave/experiments/ofinbox/internal/omnifocus"
)

func TestLiveSuggestions(t *testing.T) {
	if os.Getenv("OFINBOX_LIVE") == "" {
		t.Skip("set OFINBOX_LIVE=1 to run against real OmniFocus")
	}
	c := omnifocus.NewOsascriptClient()
	ctx := context.Background()
	hist, err := c.History(ctx)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	projects, err := c.Projects(ctx)
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	tags, err := c.Tags(ctx)
	if err != nil {
		t.Fatalf("Tags: %v", err)
	}
	inbox, err := c.InboxTasks(ctx)
	if err != nil {
		t.Fatalf("InboxTasks: %v", err)
	}
	t.Logf("history=%d projects=%d tags=%d inbox=%d", len(hist), len(projects), len(tags), len(inbox))

	l := NewLexical(hist)
	var pc, tc []Candidate
	pName := map[string]string{}
	tName := map[string]string{}
	for _, p := range projects {
		pc = append(pc, Candidate{ID: p.ID, Name: p.Name})
		pName[p.ID] = p.Name
	}
	for _, g := range tags {
		tc = append(tc, Candidate{ID: g.ID, Name: g.Name})
		tName[g.ID] = g.Name
	}
	shown := 0
	for _, task := range inbox {
		if shown >= 15 {
			break
		}
		shown++
		text := task.Name + " " + task.Note
		var ps, ts []string
		for _, id := range l.Projects(text, pc, 3) {
			ps = append(ps, pName[id])
		}
		for _, id := range l.Tags(text, tc, 3) {
			ts = append(ts, tName[id])
		}
		t.Logf("%-60.60q proj=%v tag=%v", task.Name, ps, ts)
	}
}
