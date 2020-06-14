package workerpool

import (
	"testing"
	"time"

	_ "net/http/pprof"

	"github.com/stretchr/testify/assert"
)

type HelloJob struct {
	ID string
}

func (j *HelloJob) id() string {
	return j.ID
}

func (j *HelloJob) exec() bool {
	time.Sleep(time.Second)
	return true
}

var workPoolCnt = WorkerPoolConf{
	MinCnt:        10,
	MaxCnt:        500,
	CleanInterval: 30,
}

func TestPool(t *testing.T) {
	pool := NewWorkerPool(&workPoolCnt)
	pool.Start()
	var job = HelloJob{ID: "1"}
	var index int
	for index < 1000 {
		pool.AddJob(&job)
		index++
	}
	time.Sleep(time.Second * 3)
	assert.Equal(t, int32(0), pool.JobCnt())
	pool.Stop()
}
