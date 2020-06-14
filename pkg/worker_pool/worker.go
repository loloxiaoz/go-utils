package workerpool

import (
	"time"

	logger "github.com/sirupsen/logrus"
)

var (
	//WorkerIdleTime 超过该阈值则认为worker不活跃
	WorkerIdleTime float64 = 30.0
)

type worker struct {
	jobCh       chan Job      //传递任务的channel
	stopCh      chan struct{} //传递关闭信号的channel
	lastUseTime time.Time
}

func newWorker() *worker {
	var w = worker{
		lastUseTime: time.Now(),
	}
	w.jobCh = make(chan Job)
	w.stopCh = make(chan struct{})
	return &w
}

func (w *worker) run(ch chan *worker) {
	for {
		select {
		case job := <-w.jobCh:
			job.exec()
			w.lastUseTime = time.Now()
			ch <- w
		case <-w.stopCh:
			logger.Debug("stop worker success")
			return
		}
	}
}

func (w *worker) stop() {
	w.stopCh <- struct{}{}
}

func (w *worker) handle(job Job) {
	w.jobCh <- job
}

func (w *worker) isActive() bool {
	idleTs := time.Now().Sub(w.lastUseTime).Seconds()
	if idleTs > WorkerIdleTime {
		return true
	}
	return false
}
