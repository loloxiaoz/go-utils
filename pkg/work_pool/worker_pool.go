package workpool

import (
	"sync"
	"sync/atomic"
	"time"

	logger "github.com/sirupsen/logrus"
)

//WorkerPool 工作任务池(协程池),主要目的是限制并发的协程数，控制资源的消耗
type WorkerPool struct {
	maxCnt        int32 //最大协程数
	minCnt        int32 //最小协程数
	cleanInterval int32 //清理协程间隔

	totalWorkerCnt int32     //总的worker协程数
	readyWorkerCnt int32     //当前worker协程数
	workers        []*worker //可用的worker对象

	queue    *JobQueue    //任务队列
	workerCh chan *worker //用于传递空闲的workers

	lock     sync.Mutex
	stopOnce sync.Once
	stopCh   chan struct{} //监听关闭信号的channel关闭
}

//NewWorkerPool 新建一个任务池
func NewWorkerPool(conf *WorkerPoolConf) *WorkerPool {
	var wp = WorkerPool{
		maxCnt:        conf.MaxCnt,
		minCnt:        conf.MinCnt,
		cleanInterval: conf.CleanInterval,
	}
	wp.queue = newJobQueue()
	wp.workerCh = make(chan *worker, 1000)
	wp.stopCh = make(chan struct{})
	return &wp
}

//JobCnt 获取任务池中未完成的任务数量
func (wp *WorkerPool) JobCnt() int32 {
	return wp.queue.jobCnt()
}

//AddJob 向任务池中增加任务
func (wp *WorkerPool) AddJob(job Job) {
	wp.queue.push(&job)
	logger.Debug("add 1 job into job queue, jobs:", wp.queue.cnt)
}

//BatchAddJob 向任务池中增加任务
func (wp *WorkerPool) BatchAddJob(jobs []*Job) {
	wp.queue.batchInsert(jobs)
	logger.Debug("add ", len(jobs), " job into job queue, jobs:", wp.queue.cnt)
}

//Start 初始化minCnt个协程
func (wp *WorkerPool) Start() {
	var index int32
	for index < wp.minCnt {
		wp.addWorker()
		index++
	}
	go wp.run()
}

//Stop 停止pool中的所有worker
func (wp *WorkerPool) Stop() {
	wp.stopOnce.Do(func() {
		wp.lock.Lock()
		defer wp.lock.Unlock()
		//从pool中删除worker和work计数
		atomic.StoreInt32(&wp.totalWorkerCnt, 0)
		atomic.StoreInt32(&wp.readyWorkerCnt, 0)
		for _, w := range wp.workers {
			w.stop()
		}
		wp.workers = nil
		wp.stopCh <- struct{}{}
		logger.Warn("stop worker pool success")
	})
}

//向queue尾部添加worker
func (wp *WorkerPool) addWorker() *worker {
	w := newWorker()
	atomic.AddInt32(&wp.readyWorkerCnt, 1)
	atomic.AddInt32(&wp.totalWorkerCnt, 1)
	wp.workers = append(wp.workers, w)
	logger.Debug("add 1 worker into work pool, total :", wp.totalWorkerCnt, ", ready :", wp.readyWorkerCnt)
	go w.run(wp.workerCh)
	return w
}

//从queue头部取出worker
func (wp *WorkerPool) delWorker() *worker {
	w := wp.workers[0]
	wp.workers = wp.workers[1:]
	atomic.AddInt32(&wp.readyWorkerCnt, -1)
	logger.Debug("del 1 worker from work pool, total :", wp.totalWorkerCnt, ", ready :", wp.readyWorkerCnt)
	return w
}

func (wp *WorkerPool) getWorker() (w *worker, isOk bool) {
	wp.lock.Lock()
	defer wp.lock.Unlock()
	//如果有空闲的worker，则直接返回
	if atomic.LoadInt32(&wp.readyWorkerCnt) > 0 {
		w = wp.delWorker()
		return w, true
	}
	//如果没有空闲的worker且当前总worker数量小于最大值, 则创建worker
	if atomic.LoadInt32(&wp.totalWorkerCnt) < wp.maxCnt {
		wp.addWorker()
		w = wp.delWorker()
		return w, true
	}
	return nil, false
}

func (wp *WorkerPool) clean() bool {
	defer logger.Warn("finish clean pool, total :", wp.totalWorkerCnt, ", ready :", wp.readyWorkerCnt)
	//如果小于pool最小协程数，则直接返回
	if atomic.LoadInt32(&wp.readyWorkerCnt) <= wp.minCnt {
		return false
	}
	//否则删除不活跃的worker,只保留pool最小协程数
	wp.lock.Lock()
	defer wp.lock.Unlock()
	var newWorkers []*worker
	for _, worker := range wp.workers {
		if atomic.LoadInt32(&wp.readyWorkerCnt) > wp.minCnt &&
			worker.isActive() {
			worker.stop()
			atomic.AddInt32(&wp.readyWorkerCnt, -1)
			atomic.AddInt32(&wp.totalWorkerCnt, -1)
		} else {
			newWorkers = append(newWorkers, worker)
		}
	}
	wp.workers = newWorkers
	return true
}

func (wp *WorkerPool) run() {
	ticker := time.NewTicker(time.Second * time.Duration(wp.cleanInterval))
	for {
		select {
		case <-ticker.C:
			//清理无用的worker
			wp.clean()
		case <-wp.stopCh:
			//退出pool
			ticker.Stop()
			return
		case w := <-wp.workerCh:
			//新增可用workers
			wp.lock.Lock()
			wp.workers = append(wp.workers, w)
			atomic.AddInt32(&wp.readyWorkerCnt, 1)
			wp.lock.Unlock()
		default:
			//消费job
			if wp.queue.empty() {
				time.Sleep(time.Millisecond * time.Duration(10))
				break
			}
			if w, isOk := wp.getWorker(); isOk {
				job, isOk := wp.queue.pop()
				if !isOk {
					continue
				}
				go w.handle(*job)
			}
		}
	}
}
