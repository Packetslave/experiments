package suggest

import (
	"testing"

	"github.com/packetslave/experiments/ofinbox/internal/omnifocus"
)

var projects = []Candidate{
	{ID: "p1", Name: "Home maintenance"},
	{ID: "p2", Name: "Health"},
	{ID: "p3", Name: "Travel prep"},
	{ID: "p4", Name: "Reading list"},
}

func TestHistoryMatchRanksProjectFirst(t *testing.T) {
	l := NewLexical([]omnifocus.HistoryEntry{
		{Name: "Dentist cleaning", ProjectID: "p2"},
		{Name: "Schedule annual physical", ProjectID: "p2"},
		{Name: "Replace furnace filter", ProjectID: "p1"},
	})
	got := l.Projects("Book dentist appointment", projects, 3)
	if len(got) == 0 || got[0] != "p2" {
		t.Fatalf("Projects = %v, want p2 first", got)
	}
}

func TestNameMatchWorksWithEmptyHistory(t *testing.T) {
	l := NewLexical(nil)
	got := l.Projects("annual health checkup", projects, 3)
	if len(got) != 1 || got[0] != "p2" {
		t.Fatalf("Projects = %v, want [p2]", got)
	}
}

func TestNoOverlapMeansNoSuggestions(t *testing.T) {
	l := NewLexical([]omnifocus.HistoryEntry{
		{Name: "Dentist cleaning", ProjectID: "p2"},
	})
	if got := l.Projects("buy birthday balloons", projects, 3); len(got) != 0 {
		t.Fatalf("Projects = %v, want none", got)
	}
	if got := l.Projects("", projects, 3); len(got) != 0 {
		t.Fatalf("Projects on empty text = %v, want none", got)
	}
}

func TestStrongMatchBeatsManyWeakOnes(t *testing.T) {
	// p1 has many entries sharing one common token with the query; p3 has a
	// single entry sharing two rarer tokens. The single strong precedent wins.
	l := NewLexical([]omnifocus.HistoryEntry{
		{Name: "renew library card", ProjectID: "p1"},
		{Name: "renew domain name", ProjectID: "p1"},
		{Name: "renew gym membership", ProjectID: "p1"},
		{Name: "renew passport photos", ProjectID: "p3"},
	})
	got := l.Projects("renew passport", projects, 3)
	if len(got) < 2 || got[0] != "p3" {
		t.Fatalf("Projects = %v, want p3 first", got)
	}
}

func TestLearnAffectsLaterRanking(t *testing.T) {
	l := NewLexical(nil)
	if got := l.Projects("book campsite for Yosemite", projects, 3); len(got) != 0 {
		t.Fatalf("before learning: %v, want none", got)
	}
	l.Learn(omnifocus.HistoryEntry{Name: "Reserve Yosemite permits", ProjectID: "p3"})
	got := l.Projects("book campsite for Yosemite", projects, 3)
	if len(got) != 1 || got[0] != "p3" {
		t.Fatalf("after learning: %v, want [p3]", got)
	}
}

func TestTagsRankedByHistoryAndName(t *testing.T) {
	tags := []Candidate{
		{ID: "g1", Name: "errand"},
		{ID: "g2", Name: "phone"},
	}
	l := NewLexical([]omnifocus.HistoryEntry{
		{Name: "Dentist cleaning", ProjectID: "p2", TagIDs: []string{"g2"}},
	})
	got := l.Tags("book dentist appointment", tags, 3)
	if len(got) != 1 || got[0] != "g2" {
		t.Fatalf("Tags = %v, want [g2]", got)
	}
	// Name evidence: the tag's own name in the text, no history needed.
	got = l.Tags("phone the plumber", tags, 3)
	if len(got) == 0 || got[0] != "g2" {
		t.Fatalf("Tags by name = %v, want g2 first", got)
	}
}

func TestTopNLimit(t *testing.T) {
	l := NewLexical([]omnifocus.HistoryEntry{
		{Name: "renew a", ProjectID: "p1"},
		{Name: "renew b", ProjectID: "p2"},
		{Name: "renew c", ProjectID: "p3"},
		{Name: "renew d", ProjectID: "p4"},
	})
	if got := l.Projects("renew everything", projects, 2); len(got) != 2 {
		t.Fatalf("Projects = %v, want exactly 2", got)
	}
}
