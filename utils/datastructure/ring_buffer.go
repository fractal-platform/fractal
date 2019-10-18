package datastructure

import (
	"sync"
)

var (
	mutex    sync.Mutex
	idxMutex sync.RWMutex
)

type Buffer struct {
	buf  []interface{}
	off  int
	size int
}

func New(n int) *Buffer {
	return &Buffer{
		buf:  make([]interface{}, n),
		off:  0,
		size: 0,
	}
}

func (b *Buffer) Put(v interface{}) interface{} {
	var old interface{}
	mutex.Lock()
	{
		if b.off == cap(b.buf)-1 {
			b.off = -1
		}
		b.off++
		old = b.buf[b.off]
		b.buf[b.off] = v
		if b.size != cap(b.buf) {
			b.size++
		}
	}
	mutex.Unlock()
	return old
}

func (b *Buffer) Get(n int) []interface{} {
	if n >= cap(b.buf) {
		return b.buf
	}
	res := make([]interface{}, n)
	for i := 0; i < n; i++ {
		pos := b.off - i
		if pos < 0 {
			pos += cap(b.buf)
		}
		res[i] = b.buf[pos]
	}
	return res
}

func (b *Buffer) GetOne() interface{} {
	return b.buf[b.off]
}

// Slice return a slice of buffer s[from: to]
func (b *Buffer) Slice(from int, to int) []interface{} {
	if from > to || from >= b.size || to >= b.size {
		return make([]interface{}, 0)
	}
	s := make([]interface{}, to-from+1)
	for i := 0; i < cap(s); i++ {
		pos := b.off - from - i
		if pos < 0 {
			pos += cap(b.buf)
		}
		s[i] = b.buf[pos]
	}
	return s
}

func (b *Buffer) Size() int {
	return b.size
}

type IndexBuffer struct {
	Buffer
	kBuf  *Buffer
	index map[string]*interface{}
}

func NewIndexBuf(size int) *IndexBuffer {
	return &IndexBuffer{
		Buffer: Buffer{
			buf:  make([]interface{}, size),
			size: 0,
			off:  0,
		},
		kBuf:  New(size),
		index: make(map[string]*interface{}),
	}
}

func (ib *IndexBuffer) Put(k string, v interface{}) {
	idxMutex.Lock()
	{
		oldK := ib.kBuf.Put(k)
		ib.Buffer.Put(v)
		ib.index[k] = &v
		if oldK != nil {
			delete(ib.index, oldK.(string))
		}
	}
	idxMutex.Unlock()
}

func (ib *IndexBuffer) Get(k string) *interface{} {
	idxMutex.RLock()
	v := ib.index[k]
	idxMutex.RUnlock()
	return v
}
