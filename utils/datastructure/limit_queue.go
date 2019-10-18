package datastructure

// LimitQueue represents a single instance of the limit queue data structure.
type LimitQueue struct {
	buf        []interface{}
	head, tail int
	capacity   int
}

// New constructs and returns a new LimitQueue.
func NewLimitQueue(len int) *LimitQueue {
	return &LimitQueue{
		buf: make([]interface{}, len+1),
		capacity: len,
	}
}

// Length returns the number of elements currently stored in the queue.
func (q *LimitQueue) Capacity() int {
	return q.capacity
}

// Add puts an element on the end of the queue.
func (q *LimitQueue) Add(elem interface{}) {
	q.buf[q.tail] = elem

	// move tail & head
	q.tail = (q.tail + 1) % len(q.buf)
	if q.tail == q.head {
		q.head = (q.head + 1) % len(q.buf)
	}
	//fmt.Printf("head: %d, tail: %d, len: %d\n", q.head, q.tail, len(q.buf))
}

// Peek returns the element at the head of the queue. This call panics
// if the queue is empty.
func (q *LimitQueue) Peek() interface{} {
	if q.head == q.tail {
		panic("queue: Peek() called on empty queue")
	}
	return q.buf[q.head]
}
