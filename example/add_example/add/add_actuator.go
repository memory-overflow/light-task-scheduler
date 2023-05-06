package addexample

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	framework "github.com/memory-overflow/light-task-scheduler"
)

// AddActuator 加法执行器，同步任务异步化的示例
type AddActuator struct {
	runningTask sync.Map // taskId -> framework.AsyncTaskStatus 映射
	resultMap   sync.Map // taskId -> int32 映射
}

// Init 任务在被调度前的初始化工作
func (a *AddActuator) Init(ctx context.Context, task *framework.Task) (
	newTask *framework.Task, err error) {
	// 无初始化工作
	return task, nil
}

func (add *AddActuator) work(taskId string, a, b int32) {
	time.Sleep(time.Duration(rand.Intn(4000))*time.Millisecond + 2*time.Second) // 25% 概率超时
	if _, ok := add.runningTask.Load(taskId); !ok {
		// 任务可能因为超时被删除，不处理
		return
	}
	newStatus := framework.AsyncTaskStatus{
		TaskStatus: framework.TASK_STATUS_SUCCESS,
		Progress:   100.0,
	}
	add.resultMap.Store(taskId, a+b)
	add.runningTask.Store(taskId, newStatus)
}

// Start 执行任务
func (a *AddActuator) Start(ctx context.Context, ftask *framework.Task) (
	newTask *framework.Task, ignoreErr bool, err error) {
	if _, ok := a.runningTask.Load(ftask.TaskId); ok {
		// 任务已经在执行中，不能重复执行
		return ftask, false, fmt.Errorf("task %s is running", ftask.TaskId)
	}
	task, ok := ftask.TaskItem.(AddTask)
	if !ok {
		return ftask, false, fmt.Errorf("TaskItem not be set to AddTask")
	}
	ftask.TaskStatus = framework.TASK_STATUS_RUNNING
	ftask.TaskStartTime = time.Now()
	a.runningTask.Store(ftask.TaskId, framework.AsyncTaskStatus{
		TaskStatus: framework.TASK_STATUS_RUNNING,
		Progress:   0.0,
	})
	go a.work(ftask.TaskId, task.A, task.B)
	log.Printf("start success, taskid: %s\n", ftask.TaskId)
	return ftask, false, nil
}

// Stop 停止任务
func (a *AddActuator) Stop(ctx context.Context, ftask *framework.Task) error {
	// 同步任务无法真正暂停，只能删除状态
	a.runningTask.Delete(ftask.TaskId)
	a.resultMap.Delete(ftask.TaskId)
	return nil
}

// GetAsyncTaskStatus 批量获取任务状态
func (a *AddActuator) GetAsyncTaskStatus(ctx context.Context, ftasks []framework.Task) (
	status []framework.AsyncTaskStatus, err error) {
	for _, ftask := range ftasks {
		fstatus, ok := a.runningTask.Load(ftask.TaskId)
		if !ok {
			status = append(status, framework.AsyncTaskStatus{
				TaskStatus:   framework.TASK_STATUS_FAILED,
				FailedReason: errors.New("同步任务未找到"),
				Progress:     float32(0.0),
			})
		} else {
			if fstatus.(framework.AsyncTaskStatus).TaskStatus != framework.TASK_STATUS_RUNNING {
				a.runningTask.Delete(ftask.TaskId) // delete task status after query if task finished
			}
			status = append(status, fstatus.(framework.AsyncTaskStatus))
		}
	}
	return status, nil
}

// GetOutput ...
func (a *AddActuator) GetOutput(ctx context.Context, ftask *framework.Task) (
	data interface{}, err error) {
	res, ok := a.resultMap.Load(ftask.TaskId)
	if !ok {
		return nil, fmt.Errorf("not found result for task %s", ftask.TaskId)
	}
	a.resultMap.Delete(ftask.TaskId) // delete result after get
	// reutrn int32
	return res, nil
}
