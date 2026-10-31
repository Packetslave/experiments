// Package dabalite implements DABA Lite, a sliding-window aggregation
// algorithm for first-in first-out windows. Insert, Evict and Query each make
// a constant number of calls to the aggregation operator in the worst case
// (not merely amortized), and the window uses n + 2 stored partial aggregates
// for n elements.
//
// The operator only has to be associative: it need not be commutative or
// invertible, so Max, Min, string concatenation, Bloom filters and the like
// all work.
//
// This is a Go port of DABALite.hpp and ChunkedArrayQueue.hpp from IBM's
// sliding-window-aggregators (Apache License 2.0):
// https://github.com/IBM/sliding-window-aggregators/blob/master/cpp/src/DABALite.hpp
//
// The algorithm is described in Tangwongsan, Hirzel and Schneider, "In-Order
// Sliding-Window Aggregation in Worst-Case Constant Time" (VLDB Journal, 2021).
package dabalite

// Monoid is the aggregation operator. Combine must be associative, and the
// identity passed to New must satisfy Combine(identity, x) == x ==
// Combine(x, identity).
type Monoid[In, Agg, Out any] interface {
	// Lift turns one input element into a partial aggregate.
	Lift(In) Agg
	// Combine merges two partial aggregates; older is the aggregate of
	// elements that arrived before those in newer.
	Combine(older, newer Agg) Agg
	// Lower turns a partial aggregate into a query result.
	Lower(Agg) Out
}

// Aggregate is a FIFO sliding window over elements of type In. Create one with
// New. It is not safe for concurrent use.
//
// The queue is split by four positions, begin <= l <= r <= a <= b <= end:
//
//	[begin, l)  each slot holds the aggregate from itself to b
//	[l, r)      each slot holds the aggregate from itself to r
//	[r, a)      each slot holds its own lifted value
//	[a, b)      each slot holds the aggregate from itself to b
//	[b, end)    the back: lifted values, summarized as a whole by backSum
//
// midSum is the aggregate of [r, b). Every operation moves l and a one step
// toward r, so the front [begin, b) is fully aggregated by the time the back
// has grown to match it, at which point flip starts the next round.
type Aggregate[In, Agg, Out any] struct {
	q  chunkedQueue[Agg]
	op Monoid[In, Agg, Out]

	// positions in the queue
	l, r, a, b iter[Agg]

	identity        Agg
	midSum, backSum Agg
}

// New returns an empty window that aggregates with op, whose identity element
// is identity.
func New[In, Agg, Out any](op Monoid[In, Agg, Out], identity Agg) *Aggregate[In, Agg, Out] {
	w := &Aggregate[In, Agg, Out]{
		op:       op,
		identity: identity,
		midSum:   identity,
		backSum:  identity,
	}
	w.q.init()
	w.l, w.r, w.a, w.b = w.q.begin, w.q.begin, w.q.begin, w.q.begin
	return w
}

// Size returns the number of elements in the window.
func (w *Aggregate[In, Agg, Out]) Size() int { return w.q.size }

// Insert appends v as the youngest element of the window.
func (w *Aggregate[In, Agg, Out]) Insert(v In) {
	lifted := w.op.Lift(v)
	w.backSum = w.op.Combine(w.backSum, lifted)
	w.q.pushBack(lifted)
	w.step()
}

// Evict removes the oldest element of the window. It panics if the window is
// empty.
func (w *Aggregate[In, Agg, Out]) Evict() {
	if w.q.size == 0 {
		panic("dabalite: Evict called on an empty window")
	}
	w.q.popFront()
	w.step()
}

// Query returns the aggregate of every element in the window, oldest to
// youngest. For an empty window it returns the lowered identity.
func (w *Aggregate[In, Agg, Out]) Query() Out {
	if w.q.size == 0 {
		return w.op.Lower(w.identity)
	}
	return w.op.Lower(w.op.Combine(w.alpha(), w.backSum))
}

// step does the constant amount of incremental work that keeps the front
// aggregated ahead of evictions.
func (w *Aggregate[In, Agg, Out]) step() {
	if w.l == w.b {
		w.flip()
	}

	if w.q.begin == w.b {
		// the window is empty: reset the preaggregated sums
		w.backSum, w.midSum = w.identity, w.identity
		return
	}

	if w.a != w.r {
		// a moves left
		prevDelta := w.delta()
		w.a.dec()
		*w.a.elem() = w.op.Combine(*w.a.elem(), prevDelta)
	}
	if w.l != w.r {
		// l moves right by itself until it hits r
		*w.l.elem() = w.op.Combine(*w.l.elem(), w.midSum)
		w.l.inc()
	} else {
		// l moves together with r and a
		w.l.inc()
		w.r.inc()
		w.a.inc()
		w.midSum = w.delta()
	}
}

// flip turns the back into the new front once the old front is fully
// aggregated.
func (w *Aggregate[In, Agg, Out]) flip() {
	w.l, w.r = w.q.begin, w.b
	w.a, w.b = w.q.end, w.q.end
	w.midSum = w.backSum
	w.backSum = w.identity
}

// alpha is the aggregate of the whole front, [begin, b).
func (w *Aggregate[In, Agg, Out]) alpha() Agg {
	if w.b == w.q.begin {
		return w.identity
	}
	return *w.q.begin.elem()
}

// delta is the aggregate of [a, b).
func (w *Aggregate[In, Agg, Out]) delta() Agg {
	if w.a == w.b {
		return w.identity
	}
	return *w.a.elem()
}
