// Package suggest ranks filing destinations (projects, tags) for an inbox
// item from its text. The v1 recommender is lexical: token overlap between
// the item and past filing decisions (weighted by inverse document
// frequency), plus direct matching against destination names. The
// Recommender interface exists so a model-backed implementation can replace
// it without touching the pickers.
package suggest

import (
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/packetslave/experiments/ofinbox/internal/omnifocus"
)

// Candidate is a selectable destination: a project or tag the recommender
// may suggest.
type Candidate struct {
	ID   string
	Name string
}

// Recommender ranks candidate destinations for an item's text. Both methods
// return up to n candidate IDs, best first, only for positive scores — an
// empty result means "no suggestion".
type Recommender interface {
	Projects(text string, candidates []Candidate, n int) []string
	Tags(text string, candidates []Candidate, n int) []string
	// Learn adds one filing decision to the corpus mid-session.
	Learn(e omnifocus.HistoryEntry)
}

// Weights, chosen by feel rather than science: a destination's own name is
// stronger evidence than any single similar past task, and repeated weak
// history matches should reinforce — not overtake — one strong match.
const (
	nameWeight    = 1.5
	repeatsWeight = 0.2
)

type entry struct {
	tokens    []string
	projectID string
	tagIDs    []string
}

// Lexical scores candidates by IDF-weighted token overlap with the filing
// history and with candidate names. Not safe for concurrent use.
type Lexical struct {
	entries []entry
	df      map[string]int // token -> number of history entries containing it
}

func NewLexical(history []omnifocus.HistoryEntry) *Lexical {
	l := &Lexical{df: make(map[string]int)}
	for _, e := range history {
		l.Learn(e)
	}
	return l
}

func (l *Lexical) Learn(e omnifocus.HistoryEntry) {
	toks := tokenize(e.Name)
	if len(toks) == 0 {
		return
	}
	l.entries = append(l.entries, entry{tokens: toks, projectID: e.ProjectID, tagIDs: e.TagIDs})
	for _, t := range toks {
		l.df[t]++
	}
}

func (l *Lexical) Projects(text string, candidates []Candidate, n int) []string {
	return l.rank(text, candidates, n, func(e entry) []string {
		if e.projectID == "" {
			return nil
		}
		return []string{e.projectID}
	})
}

func (l *Lexical) Tags(text string, candidates []Candidate, n int) []string {
	return l.rank(text, candidates, n, func(e entry) []string { return e.tagIDs })
}

// rank scores every candidate as history evidence (entries whose labels
// include the candidate and whose text overlaps the item's) plus name
// evidence (the candidate's own name overlapping the item's text), and
// returns the top n IDs with positive scores. Per candidate, the history
// component is its best single entry score plus a small bonus for the rest,
// so one strong precedent beats many weak ones but repetition still counts.
func (l *Lexical) rank(text string, candidates []Candidate, n int, labels func(entry) []string) []string {
	q := tokenSet(tokenize(text))
	if len(q) == 0 || n <= 0 {
		return nil
	}
	best := make(map[string]float64)
	sum := make(map[string]float64)
	for _, e := range l.entries {
		s := l.overlapScore(q, e.tokens)
		if s == 0 {
			continue
		}
		for _, id := range labels(e) {
			sum[id] += s
			if s > best[id] {
				best[id] = s
			}
		}
	}
	type scored struct {
		id    string
		score float64
	}
	var out []scored
	for _, c := range candidates {
		s := best[c.ID] + repeatsWeight*(sum[c.ID]-best[c.ID])
		s += nameWeight * l.overlapScore(q, tokenize(c.Name))
		if s > 0 {
			out = append(out, scored{c.ID, s})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].score > out[j].score })
	if len(out) > n {
		out = out[:n]
	}
	ids := make([]string, len(out))
	for i, s := range out {
		ids[i] = s.id
	}
	return ids
}

// overlapScore sums the IDF of tokens shared by the query set and doc.
func (l *Lexical) overlapScore(q map[string]bool, doc []string) float64 {
	seen := make(map[string]bool, len(doc))
	score := 0.0
	for _, t := range doc {
		if q[t] && !seen[t] {
			seen[t] = true
			score += l.idf(t)
		}
	}
	return score
}

// idf favors rare tokens. Smoothed so unseen tokens (and an empty corpus)
// still score positive — name matching must work with no history at all.
func (l *Lexical) idf(t string) float64 {
	return math.Log(1 + float64(len(l.entries)+1)/float64(1+l.df[t]))
}

// stopwords are tokens too common in task titles to carry filing signal.
var stopwords = map[string]bool{
	"the": true, "a": true, "an": true, "to": true, "for": true, "of": true,
	"in": true, "on": true, "and": true, "or": true, "with": true, "at": true,
	"from": true, "my": true, "re": true, "fwd": true, "up": true, "out": true,
}

// tokenize lowercases and splits on non-alphanumerics, dropping single
// characters and stopwords.
func tokenize(s string) []string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	out := fields[:0]
	for _, f := range fields {
		if len([]rune(f)) < 2 || stopwords[f] {
			continue
		}
		out = append(out, f)
	}
	return out
}

func tokenSet(toks []string) map[string]bool {
	set := make(map[string]bool, len(toks))
	for _, t := range toks {
		set[t] = true
	}
	return set
}

var _ Recommender = (*Lexical)(nil)
