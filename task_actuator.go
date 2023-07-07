package lighttaskscheduler

import "context"

// TaskActuator 任务执行器接口
type TaskActuator interface {

	// Init 任务在被加入调度系统前的初始化工作
	Init(ctx context.Context, task *Task) (newTask *Task, err error)

	// Start 开始执行任务，不要阻塞该方法，如果是同步任务，在单独的线程执行，执行器在内存中维护任务状态，转成异步任务，
	// 通过 GetAsyncTaskStatus 返回任务状态
	// ignoreErr 是否忽略任务调度的错误，等待恢复，如果 ignoreErr = false, Start 返回 error 任务会失败
	Start(ctx context.Context, task *Task) (newTask *Task, ignoreErr bool, err error)

	// GetOutput 从任务执行器获取任务执行的结果
	GetOutput(ctx context.Context, task *Task) (data interface{}, err error)

	// Stop 停止任务
	Stop(ctx context.Context, task *Task) error

	// GetAsyncTaskStatus 获取异步执行中的任务的状态
	GetAsyncTaskStatus(ctx context.Context, tasks []Task) (status []AsyncTaskStatus, err error)
}
