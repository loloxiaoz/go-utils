package ringbuffer

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRingMultipleInserts(t *testing.T) {
	rb := NewRingBuffer(5)

	val := []byte("1")
	isOk := rb.Put(val)
	if !assert.True(t, isOk) {
		return
	}

	val = []byte("2")
	isOk = rb.Put(val)
	if !assert.True(t, isOk) {
		return
	}

	result := rb.Get()
	assert.Equal(t, []byte("1"), result)

	result = rb.Get()
	assert.Equal(t, []byte("2"), result)
}

func TestIntertwinedGetAndPut(t *testing.T) {
	val := []byte("1")
	rb := NewRingBuffer(5)
	isOk := rb.Put(val)
	if !assert.True(t, isOk) {
		return
	}

	result := rb.Get()
	assert.Equal(t, val, result)

	val = []byte("2")
	isOk = rb.Put(val)
	if !assert.True(t, isOk) {
		return
	}

	result = rb.Get()
	assert.Equal(t, val, result)
}

func TestPutToFull(t *testing.T) {
	rb := NewRingBuffer(3)

	for i := 0; i < 4; i++ {
		isOk := rb.Put([]byte("0"))
		if !assert.True(t, isOk) {
			return
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		isOk := rb.Put([]byte("4"))
		assert.True(t, isOk)
		wg.Done()
	}()

	go func() {
		defer wg.Done()
		result := rb.Get()
		assert.Equal(t, []byte("0"), result)
	}()

	wg.Wait()
}

func TestRingGetEmpty(t *testing.T) {
	rb := NewRingBuffer(3)

	var wg sync.WaitGroup
	wg.Add(1)

	// want to kick off this consumer to ensure it blocks
	val := []byte("0")
	go func() {
		wg.Done()
		result := rb.Get()
		assert.Equal(t, val, result)
		wg.Done()
	}()

	wg.Wait()
	wg.Add(2)

	go func() {
		defer wg.Done()
		isOk := rb.Put(val)
		assert.True(t, isOk)
	}()

	wg.Wait()
}

func TestRingLen(t *testing.T) {
	rb := NewRingBuffer(4)
	assert.Equal(t, uint64(0), rb.Len())

	val := []byte("1")
	rb.Put(val)
	assert.Equal(t, uint64(1), rb.Len())

	rb.Get()
	assert.Equal(t, uint64(0), rb.Len())

	for i := 0; i < 4; i++ {
		rb.Put(val)
	}
	assert.Equal(t, uint64(4), rb.Len())

	rb.Get()
	assert.Equal(t, uint64(3), rb.Len())

	rb.Cap()
	assert.Equal(t, uint64(3), rb.Len())
}

func BenchmarkRBLifeCycle(b *testing.B) {
	rb := NewRingBuffer(64)

	counter := uint64(0)
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		for {
			rb.Get()

			if atomic.AddUint64(&counter, 1) == uint64(b.N) {
				return
			}
		}
	}()

	b.ResetTimer()

	val := []byte("1")
	for i := 0; i < b.N; i++ {
		rb.Put(val)
	}

	wg.Wait()
}

func BenchmarkRBLifeCycleContention(b *testing.B) {
	rb := NewRingBuffer(200000)

	var cnt int32
	val := []byte("1")
	for i := 0; i < 9; i++ {
		go func() {
			for {
				v := rb.Get()
				atomic.AddInt32(&cnt, 1)
				assert.Equal(b, val, v)
			}
		}()
	}

	b.ResetTimer()

	var wwg sync.WaitGroup
	wwg.Add(3)
	for i := 0; i < 3; i++ {
		go func() {
			for j := 0; j < b.N; j++ {
				rb.Put(val)
			}
			wwg.Done()
		}()
	}

	wwg.Wait()
}

func BenchmarkRBPut(b *testing.B) {
	rb := NewRingBuffer(uint64(b.N))

	b.ResetTimer()
	val := []byte("1")

	for i := 0; i < b.N; i++ {
		ok := rb.Put(val)
		if !ok {
			b.Fail()
		}
	}
}

var (
	val          = []byte("1")
	total        = 1500000
	bufferSize   = 200000
	putWorkerCnt = 3
	getWorkerCnt = 9
)

func traceMemStats() {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	fmt.Printf("Alloc:%d(bytes) HeapIdle:%d(bytes) HeapReleased:%d(bytes)\n", ms.Alloc, ms.HeapIdle, ms.HeapReleased)
}

func TestSpped(t *testing.T) {
	RingSpeedTest(t)
	ChannelSpeedTest(t)
}

func RingSpeedTest(t *testing.T) {
	runtime.GC()
	start := time.Now()
	testSpeedWithRing(t)
	end := time.Now()
	timeCost := end.Sub(start).Seconds()
	speed := float64(total) / end.Sub(start).Seconds()
	fmt.Printf("use ring buffer, total %d logs, time cost %f, read worker is %d, write worker is %d, speed is %f logs/s \n",
		total, timeCost, putWorkerCnt, getWorkerCnt, speed)
}

func ChannelSpeedTest(t *testing.T) {
	runtime.GC()
	start := time.Now()
	testSpeedWithChannel(t)
	end := time.Now()
	timeCost := end.Sub(start).Seconds()
	speed := float64(total) / timeCost
	fmt.Printf("use channel, total %d logs, time cost %f, read worker is %d, write worker is %d, speed is %f logs/s \n",
		total, timeCost, putWorkerCnt, getWorkerCnt, speed)
}

func testSpeedWithRing(t *testing.T) {
	rb := NewRingBuffer(uint64(bufferSize))
	for i := 0; i < putWorkerCnt; i++ {
		go func() {
			putCnt := int(total / putWorkerCnt)
			for j := 0; j < putCnt; j++ {
				rb.Put(val)
			}
		}()
	}

	var wg sync.WaitGroup
	wg.Add(1)
	var cnt int32
	for i := 0; i < getWorkerCnt; i++ {
		go func() {
			for {
				if atomic.LoadInt32(&cnt) == int32(total) {
					wg.Done()
					break
				}
				rb.Get()
				atomic.AddInt32(&cnt, 1)
			}
		}()
	}
	wg.Wait()
	assert.True(t, total <= int(cnt))
}

func testSpeedWithChannel(t *testing.T) {
	ch := make(chan []byte, bufferSize)
	for i := 0; i < putWorkerCnt; i++ {
		go func() {
			putCnt := int(total / putWorkerCnt)
			for j := 0; j < putCnt; j++ {
				ch <- val
			}
		}()
	}

	var wg sync.WaitGroup
	wg.Add(1)
	var cnt int32
	for i := 0; i < getWorkerCnt; i++ {
		go func() {
			for {
				<-ch
				atomic.AddInt32(&cnt, 1)
				if atomic.LoadInt32(&cnt) >= int32(total) {
					wg.Done()
					break
				}
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, total, int(cnt))
}
