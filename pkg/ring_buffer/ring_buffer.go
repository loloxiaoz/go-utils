package ringbuffer

import (
	"runtime"
	"sync/atomic"
	"time"
)

type node struct {
	position uint64
	data     []byte
}

type nodes []*node

// RingBuffer 是一个MPMC buffer, 只有原子操作并且线程安全
type RingBuffer struct {
	_padding0 [8]uint64
	in        uint64
	_padding1 [8]uint64
	out       uint64
	_padding2 [8]uint64
	mask      uint64
	_padding3 [8]uint64
	nodes     nodes
}

func (rb *RingBuffer) init(size uint64) {
	size = roundUp(size)
	rb.nodes = make(nodes, size)
	for i := uint64(0); i < size; i++ {
		rb.nodes[i] = &node{position: i}
	}
	rb.mask = size - 1
}

// Put 往buffer增加元素.  如果buffer是满的则会阻塞
func (rb *RingBuffer) Put(item []byte) bool {
	var n *node
	pos := atomic.LoadUint64(&rb.in)
	for {
		n = rb.nodes[pos&rb.mask]
		seq := atomic.LoadUint64(&n.position)
		switch dif := seq - pos; {
		//获得该item
		case dif == 0:
			if atomic.CompareAndSwapUint64(&rb.in, pos, pos+1) {
				n.data = item
				//通过+1表示该item已写入
				atomic.StoreUint64(&n.position, pos+1)
				return true
			}
		default:
			pos = atomic.LoadUint64(&rb.in)
		}
		runtime.Gosched()
	}
}

//Poll 从buffer中取下一个元素, 如果buffer是空的则会阻塞
func (rb *RingBuffer) poll(maxRetry int) []byte {
	var (
		n   *node
		pos = atomic.LoadUint64(&rb.out)
		cnt int
	)
	for {
		n = rb.nodes[pos&rb.mask]
		seq := atomic.LoadUint64(&n.position)
		//判断是否已写入
		switch dif := seq - (pos + 1); {
		case dif == 0:
			if atomic.CompareAndSwapUint64(&rb.out, pos, pos+1) {
				data := n.data
				n.data = nil
				atomic.StoreUint64(&n.position, pos+rb.mask+1)
				return data
			}
		default:
			pos = atomic.LoadUint64(&rb.out)
		}
		if maxRetry > 0 && cnt >= maxRetry {
			return nil
		}
		cnt++
		time.Sleep(time.Millisecond)
	}
}

//Get 从buffer中取下一个元素, 如果buffer是空的则会阻塞
func (rb *RingBuffer) Get() []byte {
	return rb.poll(0)
}

//Poll 从buffer中取下一个元素, 如果buffer是空的则会阻塞
func (rb *RingBuffer) Poll() []byte {
	return rb.poll(10)
}

//Len 当前ring_buffer长度
func (rb *RingBuffer) Len() uint64 {
	return atomic.LoadUint64(&rb.in) - atomic.LoadUint64(&rb.out)
}

//Cap 当前ring_buffer的容量
func (rb *RingBuffer) Cap() uint64 {
	return uint64(len(rb.nodes))
}

//NewRingBuffer 新建一个ring_buffer
func NewRingBuffer(size uint64) *RingBuffer {
	rb := &RingBuffer{}
	rb.init(size)
	return rb
}
