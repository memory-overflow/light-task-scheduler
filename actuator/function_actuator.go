// 封装好的函数执行器

package actuator

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	framework "github.com/memory-overflow/light-task-scheduler"
	"github.com/patrickmn/go-cache"
)

// fucntionActuator 函数执行器，同步任务异步化的示例
type fucntionActuator struct {
	runFunc         RunFunction         // 执行函数
	initFunc        InitFunction        // 初始函数
	callbackChannel chan framework.Task // 回调队列

	runningTask *cache.Cache // taskId -> [framework.AsyncTaskStatus, cancel function] 映射
	datatMap    *cache.Cache // taskId -> interface{} 映射

}

// RunFunction 执行函数的定义，实现该函数的时候，建议使用传入 ctx 进行超时和退出处理
// data 任务执行完成返回的数据
type RunFunction func(ctx context.Context, task *framework.Task) (data interface{}, err error)

// InitFunction 任务执行前的初始工作
type InitFunction func(ctx context.Context, task *framework.Task) (newTask *framework.Task, err error)

// runFunc 待调度的执行函数，注意实现该函数的时候，需要使用传入 ctx 进行超时和退出处理，框架否则无法控制超时时间
// callbackChannel 用来执行器进行任务回调，返回已经完成的任务，如果不需要回调，传入 nil 即可
func MakeFucntionActuator(runFunc RunFunction, initFunc InitFunction) (*fucntionActuator, error) {
	if runFunc == nil {
		return nil, fmt.Errorf("runFunc is nil")
	}
	return &fucntionActuator{
		runFunc:     runFunc,
		initFunc:    initFunc,
		runningTask: cache.New(5*24*time.Hour, 24*time.Hour), // 缓存5天
		datatMap:    cache.New(5*24*time.Hour, 24*time.Hour), // 缓存5天
	}, nil
}

// SetCallbackChannel 任务配置回调 channel
func (fc *fucntionActuator) SetCallbackChannel(callbackChannel chan framework.Task) {
	fc.callbackChannel = callbackChannel
}

// Init 任务在被调度前的初始化工作
func (fc *fucntionActuator) Init(ctx context.Context, task *framework.Task) (
	newTask *framework.Task, err error) {
	if fc.initFunc != nil {
		return fc.initFunc(ctx, task)
	}
	return task, nil
}

// Start 执行任务
func (fc *fucntionActuator) Start(ctx context.Context, ftask *framework.Task) (
	newTask *framework.Task, ignoreErr bool, err error) {
	if st, ok := fc.runningTask.Get(ftask.TaskId); ok {
		status := st.([]interface{})[0].(framework.AsyncTaskStatus).TaskStatus
		if status == framework.TASK_STATUS_RUNNING || status == framework.TASK_STATUS_SUCCESS {
			// 任务已经在执行中，不能重复执行
			return ftask, false, fmt.Errorf("task %s is running", ftask.TaskId)
		}
	}
	runCtx, cancel := context.WithCancel(ctx)

	ftask.TaskStatus = framework.TASK_STATUS_RUNNING
	ftask.TaskStartTime = time.Now()
	fc.datatMap.Delete(ftask.TaskId)
	fc.runningTask.Set(ftask.TaskId,
		[]interface{}{
			framework.AsyncTaskStatus{
				TaskStatus: framework.TASK_STATUS_RUNNING,
				Progress:   0.0,
			}, cancel}, cache.DefaultExpiration)

	go func() {
		data, err := func() (data interface{}, err error) {
			defer func() {
				if p := recover(); p != nil {
					data = nil

					err = fmt.Errorf("panic: %v, stacktrace: %s", p, debug.Stack())
				}
			}()
			return fc.runFunc(runCtx, ftask)
		}()
		st, ok := fc.runningTask.Get(ftask.TaskId)
		if !ok {
			// 任务可能因为超时被删除，或者手动暂停、不处理
			return
		}
		if st.([]interface{})[1] != nil {
			(st.([]interface{})[1].(context.CancelFunc))() // 任务执行完，也要执行对应的 cancel 函数
		}
		var newStatus framework.AsyncTaskStatus
		if err != nil {
			newStatus = framework.AsyncTaskStatus{
				TaskStatus:   framework.TASK_STATUS_FAILED,
				Progress:     0.0,
				FailedReason: err,
			}
		} else {
			newStatus = framework.AsyncTaskStatus{
				TaskStatus: framework.TASK_STATUS_SUCCESS,
				Progress:   100.0,
			}
			fc.datatMap.Set(ftask.TaskId, data, cache.DefaultExpiration) // 先存结果
		}
		fc.runningTask.Set(ftask.TaskId, []interface{}{newStatus, nil}, cache.DefaultExpiration)
		if fc.callbackChannel != nil {
			// 如果需要回调
			callbackTask := *ftask
			callbackTask.TaskStatus = newStatus.TaskStatus
			callbackTask.TaskEnbTime = time.Now()
			if newStatus.FailedReason != nil {
				callbackTask.FailedReason = newStatus.FailedReason
			}
			fc.callbackChannel <- callbackTask
		}
	}()

	return ftask, false, nil
}

func (fc *fucntionActuator) clear(taskId string) {
	fc.runningTask.Delete(taskId)
	fc.datatMap.Delete(taskId)
}

// Stop 停止任务
func (fc *fucntionActuator) Stop(ctx context.Context, ftask *framework.Task) error {
	st, ok := fc.runningTask.Get(ftask.TaskId)
	if !ok {
		// 未找到任务
		fc.datatMap.Delete(ftask.TaskId)
		return nil
	}
	fc.clear(ftask.TaskId)
	if st.([]interface{})[1] != nil {
		(st.([]interface{})[1].(context.CancelFunc))() // cancel 函数 ctx
	}
	return nil
}

// GetAsyncTaskStatus 批量获取任务状态
func (fc *fucntionActuator) GetAsyncTaskStatus(ctx context.Context, ftasks []framework.Task) (
	status []framework.AsyncTaskStatus, err error) {
	for _, ftask := range ftasks {
		fstatus, ok := fc.runningTask.Get(ftask.TaskId)
		if !ok {
			status = append(status, framework.AsyncTaskStatus{
				TaskStatus:   framework.TASK_STATUS_FAILED,
				FailedReason: errors.New("同步任务未找到"),
				Progress:     float32(0.0),
			})
		} else {
			st := fstatus.([]interface{})[0].(framework.AsyncTaskStatus)
			if st.TaskStatus != framework.TASK_STATUS_RUNNING {
				fc.runningTask.Delete(ftask.TaskId) // delete task status after query if task finished
			}
			status = append(status, st)
		}
	}
	return status, nil
}

// GetOutput ...
func (fc *fucntionActuator) GetOutput(ctx context.Context, ftask *framework.Task) (
	data interface{}, err error) {
	res, ok := fc.datatMap.Get(ftask.TaskId)
	if !ok {
		return nil, fmt.Errorf("not found result for task %s", ftask.TaskId)
	}
	fc.clear(ftask.TaskId)
	return res, nil
}
