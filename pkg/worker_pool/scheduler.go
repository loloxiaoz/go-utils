package workerpool

import (
	"time"

	logger "github.com/sirupsen/logrus"
)

//TasksPackAction  任务集动作
type TasksPackAction string

var (
	//AddTasks 新增任务集
	AddTasks TasksPackAction = "add"
	//DelTasks 删除任务集
	DelTasks TasksPackAction = "delete"
)

//TasksPack 同一类任务的包
type TasksPack struct {
	Name   string
	Tasks  map[string]*Task
	Action TasksPackAction
}

//NewTaskPack 新建同一类任务包
func NewTaskPack(name string) *TasksPack {
	var tp TasksPack
	tp.Name = name
	tp.Tasks = make(map[string]*Task)
	return &tp
}

//Scheduler 调度器
type Scheduler struct {
	tasks      map[string]map[string]*Task
	workerPool *WorkerPool

	taskCh chan TasksPack
	stopCh chan struct{}
}

//NewScheduler 运行调度器初始化
func NewScheduler(conf *WorkerPoolConf) *Scheduler {
	var s Scheduler
	s.tasks = make(map[string]map[string]*Task)
	s.stopCh = make(chan struct{})
	s.taskCh = make(chan TasksPack)
	s.workerPool = NewWorkerPool(conf)
	return &s
}

//Run 运行调度器运行
func (s *Scheduler) Run() {
	s.workerPool.Start()
	ticker := time.NewTicker(time.Second)
	for {
		select {
		case tp := (<-s.taskCh):
			if len(tp.Tasks) > 0 {
				//修改任务集数组
				s.tasks[tp.Name] = tp.Tasks
			} else {
				//删除任务集数组
				if tp.Action == DelTasks {
					delete(s.tasks, tp.Name)
				}
			}
		case <-ticker.C:
			//定时将任务放入执行池中
			ts := time.Now()
			for _, tasks := range s.tasks {
				for _, task := range tasks {
					if task.isReady(ts) {
						s.workerPool.BatchAddJob(task.GetJobs())
						task.run()
					}
				}
			}
			logger.Info("schedule tasks success")
		case <-s.stopCh:
			ticker.Stop()
			s.workerPool.Stop()
			logger.Warn("stop scheduler success")
			return
		}
	}
}

//AddTask 向调度器添加任务包
func (s *Scheduler) AddTask(tp *TasksPack) {
	s.taskCh <- *tp
}

//Stop 停止调度器运行
func (s *Scheduler) Stop() {
	s.stopCh <- struct{}{}
}
