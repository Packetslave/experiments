package dabalite

import "unsafe"

// chunkBytes is the target size of one chunk's element storage. Each chunk
// holds chunkBytes/sizeof(E) elements (at least one).
const chunkBytes = 512

type chunk[E any] struct {
	prev, next *chunk[E]
	elems      []E
}

// iter is a position in a chunkedQueue. Two iters are equal (==) exactly when
// they denote the same slot. An iter stays valid across pushBack, and across
// popFront for as long as its slot has not been popped.
type iter[E any] struct {
	c *chunk[E]
	i int
}

func (it *iter[E]) inc() {
	it.i++
	if it.i == len(it.c.elems) {
		it.c, it.i = it.c.next, 0
	}
}

func (it *iter[E]) dec() {
	if it.i == 0 {
		it.c = it.c.prev
		it.i = len(it.c.elems) - 1
	} else {
		it.i--
	}
}

func (it iter[E]) elem() *E { return &it.c.elems[it.i] }

// chunkedQueue is a FIFO queue stored as a doubly linked list of fixed-size
// chunks. Unlike a growable slice or ring buffer it never copies elements, so
// pushBack and popFront are O(1) in the worst case, not just amortized, and
// positions held in iters are never invalidated by growth.
//
// end always denotes a real, allocated slot (the one the next pushBack fills),
// so an iter equal to end becomes the position of the next pushed element.
type chunkedQueue[E any] struct {
	begin, end iter[E]
	size       int
	chunkCap   int
}

func (q *chunkedQueue[E]) init() {
	var zero E
	q.chunkCap = max(1, chunkBytes/max(1, int(unsafe.Sizeof(zero))))
	c := &chunk[E]{elems: make([]E, q.chunkCap)}
	q.begin = iter[E]{c, 0}
	q.end = iter[E]{c, 0}
}

func (q *chunkedQueue[E]) pushBack(e E) {
	*q.end.elem() = e
	q.end.i++
	if q.end.i == q.chunkCap {
		c := &chunk[E]{prev: q.end.c, elems: make([]E, q.chunkCap)}
		q.end.c.next = c
		q.end = iter[E]{c, 0}
	}
	q.size++
}

// popFront removes the front element. The queue must be non-empty.
func (q *chunkedQueue[E]) popFront() {
	var zero E
	*q.begin.elem() = zero // drop the reference so the GC can reclaim it
	q.begin.i++
	if q.begin.i == q.chunkCap {
		next := q.begin.c.next
		next.prev = nil
		q.begin = iter[E]{next, 0}
	}
	q.size--
}
