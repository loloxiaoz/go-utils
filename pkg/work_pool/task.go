package workpool

import (
	"math"
	"time"
)

//Task 任务实体
type Task struct {
	id          string
	interval    float64
	jobs        map[string]Job
	lastRunTime time.Time
}

func newTask(id string, interval float64) *Task {
	return &Task{
		id:       id,
		interval: interval,
	}
}

func (t *Task) isReady(ts time.Time) bool {
	interval := ts.Sub(t.lastRunTime).Seconds()
	if int(math.Round(interval)) >= int(t.interval) {
		return true
	}
	return false
}

func (t *Task) run() {
	t.lastRunTime = time.Now()
}

func (t *Task) GetJobs() []*Job {
	var jobs []*Job
	for name := range t.jobs {
		tmpJob := t.jobs[name]
		jobs = append(jobs, &tmpJob)
	}
	return jobs
}
