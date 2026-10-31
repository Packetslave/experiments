package dabalite

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"testing"
)

type sum struct{}

func (sum) Lift(v int) int       { return v }
func (sum) Combine(a, b int) int { return a + b }
func (sum) Lower(a int) int      { return a }

// concat is associative but not commutative, so it catches any mistake in the
// order partial aggregates are combined.
type concat struct{}

func (concat) Lift(v int) string          { return fmt.Sprintf("%d,", v) }
func (concat) Combine(a, b string) string { return a + b }
func (concat) Lower(a string) string      { return a }

// maxOp is not invertible. Its partial is the lifted value itself.
type maxOp struct{}

func (maxOp) Lift(v int) int       { return v }
func (maxOp) Combine(a, b int) int { return max(a, b) }
func (maxOp) Lower(a int) int      { return a }

// wide has a partial larger than chunkBytes, which forces one element per
// chunk and so exercises every chunk-boundary path on every operation.
type wide struct{}

type widePartial struct {
	n   int
	pad [100]int64
}

func (wide) Lift(v int) widePartial { return widePartial{n: v} }
func (wide) Combine(a, b widePartial) widePartial {
	return widePartial{n: a.n + b.n}
}
func (wide) Lower(a widePartial) int { return a.n }

// checkInvariants is the port of assert_ps_invariants from DABALite.hpp.
func checkInvariants[In, Agg, Out any](t *testing.T, w *Aggregate[In, Agg, Out]) {
	t.Helper()
	pos := func(target iter[Agg]) int {
		it := w.q.begin
		for n := 0; ; n++ {
			if it == target {
				return n
			}
			if it == w.q.end {
				t.Fatalf("position %+v is outside [begin, end]", target)
			}
			it.inc()
		}
	}
	l, r, a, b, end := pos(w.l), pos(w.r), pos(w.a), pos(w.b), pos(w.q.end)
	if end != w.q.size {
		t.Fatalf("walked %d slots, size is %d", end, w.q.size)
	}
	if !(l <= r && r <= a && a <= b && b <= end) {
		t.Fatalf("positions out of order: l=%d r=%d a=%d b=%d end=%d", l, r, a, b, end)
	}
	unprocessed, front, back := b-l, b, end-b
	if end != 0 && unprocessed+1 != front-back {
		t.Fatalf("unprocessed=%d front=%d back=%d: want unprocessed+1 == front-back", unprocessed, front, back)
	}
	if back > front {
		t.Fatalf("back=%d larger than front=%d", back, front)
	}
	if r-l != a-r {
		t.Fatalf("l..r=%d differs from r..a=%d", r-l, a-r)
	}
}

// runAgainstNaive drives a window with a random insert/evict sequence and
// checks every query against a from-scratch fold over a plain slice.
func runAgainstNaive[Agg, Out any](t *testing.T, op Monoid[int, Agg, Out], identity Agg, steps int) {
	t.Helper()
	rng := rand.New(rand.NewPCG(1, 2))
	w := New(op, identity)
	var naive []int

	for i := range steps {
		// Alternate between growing and shrinking phases so the window
		// repeatedly crosses chunk boundaries and drains to empty.
		growing := (i/1500)%2 == 0
		insert := rng.IntN(10) < 7
		if !growing {
			insert = !insert
		}
		if len(naive) == 0 {
			insert = true
		}

		if insert {
			v := rng.IntN(1000)
			w.Insert(v)
			naive = append(naive, v)
		} else {
			w.Evict()
			naive = naive[1:]
		}

		checkInvariants(t, w)
		if w.Size() != len(naive) {
			t.Fatalf("step %d: Size() = %d, want %d", i, w.Size(), len(naive))
		}
		acc := identity
		for _, v := range naive {
			acc = op.Combine(acc, op.Lift(v))
		}
		if got, want := w.Query(), op.Lower(acc); !reflect.DeepEqual(got, want) {
			t.Fatalf("step %d (size %d): Query() = %v, want %v", i, len(naive), got, want)
		}
	}
}

func TestAgainstNaive(t *testing.T) {
	t.Run("sum", func(t *testing.T) { runAgainstNaive(t, sum{}, 0, 6000) })
	t.Run("concat", func(t *testing.T) { runAgainstNaive(t, concat{}, "", 6000) })
	t.Run("max", func(t *testing.T) { runAgainstNaive(t, maxOp{}, -1, 6000) })
	t.Run("wide", func(t *testing.T) { runAgainstNaive(t, wide{}, widePartial{}, 3000) })
}

func TestFixedSizeWindow(t *testing.T) {
	const window = 100
	w := New(concat{}, "")
	for i := range 1000 {
		if w.Size() == window {
			w.Evict()
		}
		w.Insert(i)
		checkInvariants(t, w)

		want := ""
		for v := max(0, i-window+1); v <= i; v++ {
			want += fmt.Sprintf("%d,", v)
		}
		if got := w.Query(); got != want {
			t.Fatalf("after inserting %d: Query() = %q, want %q", i, got, want)
		}
	}
}

func TestEmpty(t *testing.T) {
	w := New(sum{}, 0)
	if got := w.Query(); got != 0 {
		t.Fatalf("Query() on a new window = %d, want 0", got)
	}

	w.Insert(5)
	w.Evict()
	if got := w.Query(); got != 0 || w.Size() != 0 {
		t.Fatalf("drained window: Query() = %d, Size() = %d, want 0, 0", got, w.Size())
	}

	defer func() {
		if recover() == nil {
			t.Fatal("Evict on an empty window did not panic")
		}
	}()
	w.Evict()
}

// countingSum counts Combine calls to check the worst-case bound.
type countingSum struct{ calls *int }

func (countingSum) Lift(v int) int { return v }
func (c countingSum) Combine(a, b int) int {
	*c.calls++
	return a + b
}
func (countingSum) Lower(a int) int { return a }

func TestConstantCombinesPerOperation(t *testing.T) {
	calls := 0
	w := New(countingSum{&calls}, 0)
	rng := rand.New(rand.NewPCG(3, 4))
	worst := 0
	for range 20000 {
		calls = 0
		if w.Size() == 0 || rng.IntN(10) < 6 {
			w.Insert(1)
		} else {
			w.Evict()
		}
		w.Query()
		worst = max(worst, calls)
	}
	// Insert: 1 for backSum + at most 2 in step; Query: 1.
	if worst > 4 {
		t.Fatalf("an operation plus a query made %d Combine calls, want at most 4", worst)
	}
}

func Example() {
	w := New(maxOp{}, 0)
	for _, v := range []int{3, 9, 4, 1} {
		w.Insert(v)
	}
	fmt.Println(w.Query())
	w.Evict() // 3
	w.Evict() // 9
	fmt.Println(w.Query())
	// Output:
	// 9
	// 4
}
