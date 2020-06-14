package workerpool

import (
	"sync"
	"sync/atomic"
)

//Job 任务
type Job interface {
	id() string
	exec() bool
}

//JobQueue 任务队列
type JobQueue struct {
	jobs  []*Job
	cnt   int32
	mutex sync.Mutex
}

func newJobQueue() *JobQueue {
	var queue JobQueue
	return &queue
}

func (q *JobQueue) push(job *Job) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	atomic.AddInt32(&q.cnt, 1)
	q.jobs = append(q.jobs, job)
}

func (q *JobQueue) batchInsert(jobs []*Job) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	atomic.AddInt32(&q.cnt, int32(len(jobs)))
	q.jobs = append(q.jobs, jobs...)
}

func (q *JobQueue) pop() (job *Job, isOk bool) {
	if atomic.LoadInt32(&q.cnt) <= 0 {
		return nil, false
	}
	q.mutex.Lock()
	defer q.mutex.Unlock()
	atomic.AddInt32(&q.cnt, -1)
	job = q.jobs[0]
	q.jobs = q.jobs[1:]
	return job, true
}

func (q *JobQueue) empty() bool {
	return atomic.LoadInt32(&q.cnt) <= 0
}

func (q *JobQueue) jobCnt() int32 {
	return atomic.LoadInt32(&q.cnt)
}
