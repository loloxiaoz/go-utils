package ratelimit

import (
	"sync"
	"time"
)

//Bucket 令牌桶算法
type Bucket struct {
	clock Clock

	//bucket的创建时间
	startTime time.Time

	//容量
	capacity int64

	//每tick的往bucket中添加的量
	quantum int64

	//两个tick之间的时间间隔
	fillInterval time.Duration

	mu sync.Mutex

	//最近一个tick可用的token数量
	//当有consumer等待token时，将为负数
	availableTokens int64

	// 最近一个记录了bucket中有多少token的tick
	latestTick int64
}

//NewBucket 返回一个token bucket，该bucket以固定间隔往bucket中填充token, 直到达到最大容量
//bucket初始化时是满的
func NewBucket(fillInterval time.Duration, capacity int64) *Bucket {
	return NewBucketWithClock(fillInterval, capacity, nil)
}

//NewBucketWithClock 允许传入一个clock类用于模拟时间流逝，如果clock为空，系统的clock将被使用
func NewBucketWithClock(fillInterval time.Duration, capacity int64, clock Clock) *Bucket {
	return NewBucketWithQuantumAndClock(fillInterval, capacity, 1, clock)
}

//NewBucketWithQuantumAndClock 允许传入一个clock类用于模拟时间流逝，如果clock为空，系统的clock将被使用
func NewBucketWithQuantumAndClock(fillInterval time.Duration, capacity, quantum int64, clock Clock) *Bucket {
	if clock == nil {
		clock = realClock{}
	}
	if fillInterval <= 0 {
		panic("token bucket fill interval is not > 0")
	}
	if capacity <= 0 {
		panic("token bucket capacity is not > 0")
	}
	if quantum <= 0 {
		panic("token bucket quantum is not > 0")
	}
	return &Bucket{
		clock:           clock,
		startTime:       clock.Now(),
		latestTick:      0,
		fillInterval:    fillInterval,
		capacity:        capacity,
		quantum:         quantum,
		availableTokens: 0,
	}
}

//Wait 等待bucket中有足够的tokens可用
func (tb *Bucket) Wait(count int64) {
	if d := tb.Take(count); d > 0 {
		tb.clock.Sleep(d)
	}
}

//WaitMaxDuration 判断需要等待的时间是否大于maxWait,如果大于maxWait的时间，则直接返回,否则将等待
func (tb *Bucket) WaitMaxDuration(count int64, maxWait time.Duration) bool {
	d, ok := tb.TakeMaxDuration(count, maxWait)
	if d > 0 {
		tb.clock.Sleep(d)
	}
	return ok
}

const infinityDuration time.Duration = 0x7fffffffffffffff

//Take 将从bucket中无阻塞的拿走count个token
//该方法一旦调用，将无法撤销T
func (tb *Bucket) Take(count int64) time.Duration {
	//	tb.mu.Lock()
	//	defer tb.mu.Unlock()
	d, _ := tb.take(tb.clock.Now(), count, infinityDuration)
	return d
}

//TakeMaxDuration 如果等待时间小于maxWait时间, 将从bucket中拿走指定个token,
func (tb *Bucket) TakeMaxDuration(count int64, maxWait time.Duration) (time.Duration, bool) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return tb.take(tb.clock.Now(), count, maxWait)
}

// TakeAvailable 将从bucket中拿走可用的token，将返回被拿走的token，如果没有可用的token时，将返回0(非阻塞)
func (tb *Bucket) TakeAvailable(count int64) int64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return tb.takeAvailable(tb.clock.Now(), count)
}

func (tb *Bucket) takeAvailable(now time.Time, count int64) int64 {
	if count <= 0 {
		return 0
	}
	tb.adjustavailableTokens(tb.currentTick(now))
	if tb.availableTokens <= 0 {
		return 0
	}
	if count > tb.availableTokens {
		count = tb.availableTokens
	}
	tb.availableTokens -= count
	return count
}

// Available 返回当前可用的token数, 如果有consumer等待，则返回负数
// 该函数是为了输出指标和调试用
func (tb *Bucket) Available() int64 {
	return tb.available(tb.clock.Now())
}

func (tb *Bucket) available(now time.Time) int64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.adjustavailableTokens(tb.currentTick(now))
	return tb.availableTokens
}

// Capacity 返回bucket容量
func (tb *Bucket) Capacity() int64 {
	return tb.capacity
}

// Rate 返回bucket添加的速率
func (tb *Bucket) Rate() float64 {
	return 1e9 * float64(tb.quantum) / float64(tb.fillInterval)
}

func (tb *Bucket) take(now time.Time, count int64, maxWait time.Duration) (time.Duration, bool) {
	if count <= 0 {
		return 0, true
	}

	tick := tb.currentTick(now)
	tb.adjustavailableTokens(tick)
	avail := tb.availableTokens - count
	if avail >= 0 {
		tb.availableTokens = avail
		return 0, true
	}

	endTick := tick + (-avail+tb.quantum-1)/tb.quantum
	endTime := tb.startTime.Add(time.Duration(endTick) * tb.fillInterval)
	waitTime := endTime.Sub(now)
	if waitTime > maxWait {
		return 0, false
	}
	tb.availableTokens = avail
	return waitTime, true
}

func (tb *Bucket) currentTick(now time.Time) int64 {
	return int64(now.Sub(tb.startTime) / tb.fillInterval)
}

// 根据时间计算和调整bucket中可用的token数量
func (tb *Bucket) adjustavailableTokens(tick int64) {
	lastTick := tb.latestTick
	tb.latestTick = tick
	if tb.availableTokens >= tb.capacity {
		return
	}
	tb.availableTokens += (tick - lastTick) * tb.quantum
	if tb.availableTokens > tb.capacity {
		tb.availableTokens = tb.capacity
	}
	return
}

// Clock 计时器接口
type Clock interface {
	// Now 返回当前时间
	Now() time.Time
	// Sleep 实现sleep方法
	Sleep(d time.Duration)
}

type realClock struct{}

// Now 返回当前时间
func (realClock) Now() time.Time {
	return time.Now()
}

// Sleep 实现sleep方法
func (realClock) Sleep(d time.Duration) {
	time.Sleep(d)
}
