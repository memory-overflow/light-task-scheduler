package memeorycontainer

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	lighttaskscheduler "github.com/memory-overflow/light-task-scheduler"
)

// queueContainer 队列型容器，任务无状态，无优先级，先进先出，任务数据，多进程数据无法共享数据
type queueContainer struct {
	MemeoryContainer

	stopedTaskMap    sync.Map // 需要停止的任务 map，用来标记等待中的哪些任务被停职了，taskId -> lighttaskscheduler.Task
	runningTaskMap   sync.Map // 运行中的任务的 map， taskId -> lighttaskscheduler.Task
	runningTaskCount int32    // 运行中的任务总数

	waitingTasks chan lighttaskscheduler.Task
	timeout      time.Duration
}

// MakeQueueContainer 构造队列型任务容器, size 表示队列的大小, timeout 表示队列读取的超时时间
func MakeQueueContainer(size uint32, timeout time.Duration) *queueContainer {
	return &queueContainer{
		waitingTasks: make(chan lighttaskscheduler.Task, size),
		timeout:      timeout,
	}
}

// AddTask 添加任务
func (q *queueContainer) AddTask(ctx context.Context, task lighttaskscheduler.Task) (err error) {
	// 如果是之前已经暂停，那么直接删除暂停状态
	if _, ok := q.stopedTaskMap.LoadAndDelete(task.TaskId); ok {
		return
	}
	task.TaskStatus = lighttaskscheduler.TASK_STATUS_WAITING
	select {
	case q.waitingTasks <- task:
		return
	case <-time.After(q.timeout):
		return fmt.Errorf("add task timeout")
	}
}

// AddRunningTask 添加正在分析中的任务，用于从持久化容器中恢复数据
func (q *queueContainer) AddRunningTask(ctx context.Context, task lighttaskscheduler.Task) (err error) {
	// 如果任务没有在执行列表中，加入执行列表
	if _, ok := q.runningTaskMap.LoadOrStore(task.TaskId, task); !ok {
		atomic.AddInt32(&q.runningTaskCount, 1)
	}
	return nil
}

// GetRunningTask 获取运行中的任务
func (q *queueContainer) GetRunningTask(ctx context.Context) (tasks []lighttaskscheduler.Task, err error) {
	q.runningTaskMap.Range(
		func(key, value interface{}) bool {
			task := value.(lighttaskscheduler.Task)
			tasks = append(tasks, task)
			return true
		})
	// if len(tasks) > 0 {
	// 	log.Printf("get %d task success \n", len(tasks))
	// }
	return tasks, err
}

// GetRunningTaskCount 获取运行中的任务数
func (q *queueContainer) GetRunningTaskCount(ctx context.Context) (count int32, err error) {
	return q.runningTaskCount, nil
}

// GetWaitingTask 获取等待中的任务
func (q *queueContainer) GetWaitingTask(ctx context.Context, limit int32) (tasks []lighttaskscheduler.Task, err error) {
	for i := 0; i < int(limit); i++ {
		select {
		case task := <-q.waitingTasks:
			// 暂停的任务直接忽略
			if _, ok := q.stopedTaskMap.LoadAndDelete(task.TaskId); !ok {
				tasks = append(tasks, task)
			}
		case <-time.After(q.timeout):
			return tasks, nil
		}
	}
	return tasks, nil
}

// ToRunningStatus 转移到运行中的状态
func (q *queueContainer) ToRunningStatus(ctx context.Context, task *lighttaskscheduler.Task) (
	newTask *lighttaskscheduler.Task, err error) {
	task.TaskStartTime = time.Now()
	task.TaskStatus = lighttaskscheduler.TASK_STATUS_RUNNING
	if _, ok := q.runningTaskMap.LoadOrStore(task.TaskId, *task); !ok {
		atomic.AddInt32(&q.runningTaskCount, 1)
	}
	return task, nil
}

// ToExportStatus 转移到停止状态
func (q *queueContainer) ToStopStatus(ctx context.Context, task *lighttaskscheduler.Task) (
	newTask *lighttaskscheduler.Task, err error) {
	// 如果任务已经在执行中，删除执行中的任务
	if _, ok := q.runningTaskMap.LoadAndDelete(task.TaskId); ok {
		atomic.AddInt32(&q.runningTaskCount, -1)
	} else {
		// 任务在等待队列中，任务加入到停止列表，待调度到的时候 pass 掉
		q.stopedTaskMap.Store(task.TaskId, struct{}{})
	}
	task.TaskStatus = lighttaskscheduler.TASK_STATUS_STOPED
	return task, nil
}

// ToExportStatus 转移到删除状态
func (q *queueContainer) ToDeleteStatus(ctx context.Context, task *lighttaskscheduler.Task) (
	newTask *lighttaskscheduler.Task, err error) {
	// 如果任务已经在执行中，删除执行中的任务
	if _, ok := q.runningTaskMap.LoadAndDelete(task.TaskId); ok {
		atomic.AddInt32(&q.runningTaskCount, -1)
	} else {
		// 任务在等待队列中，任务加入到暂停列表，待调度到的时候 pass 掉
		q.stopedTaskMap.Store(task.TaskId, struct{}{})
	}
	task.TaskStatus = lighttaskscheduler.TASK_STATUS_DELETE
	return task, nil
}

// ToFailedStatus 转移到失败状态
func (q *queueContainer) ToFailedStatus(ctx context.Context, task *lighttaskscheduler.Task, reason error) (
	newTask *lighttaskscheduler.Task, err error) {
	if _, ok := q.runningTaskMap.LoadAndDelete(task.TaskId); ok {
		atomic.AddInt32(&q.runningTaskCount, -1)
	}
	log.Printf("failed task %s, reason %v", task.TaskId, reason)
	task.TaskStatus = lighttaskscheduler.TASK_STATUS_FAILED
	return
}

// ToExportStatus 转移到数据导出状态
func (q *queueContainer) ToExportStatus(ctx context.Context, task *lighttaskscheduler.Task) (
	newTask *lighttaskscheduler.Task, err error) {
	// 删除执行中的任务，增加一个任务调度空位
	if _, ok := q.runningTaskMap.LoadAndDelete(task.TaskId); ok {
		atomic.AddInt32(&q.runningTaskCount, -1)
	}
	task.TaskStatus = lighttaskscheduler.TASK_STATUS_EXPORTING
	return task, nil
}

// ToSuccessStatus 转移到执行成功状态
func (q *queueContainer) ToSuccessStatus(ctx context.Context, task *lighttaskscheduler.Task) (
	newTask *lighttaskscheduler.Task, err error) {
	task.TaskStatus = lighttaskscheduler.TASK_STATUS_SUCCESS
	return task, nil
}

// UpdateRunningTaskStatus 更新执行中的任务状态
func (q *queueContainer) UpdateRunningTaskStatus(ctx context.Context,
	task *lighttaskscheduler.Task, status lighttaskscheduler.AsyncTaskStatus) error {
	return nil
}
