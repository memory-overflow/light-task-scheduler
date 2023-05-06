package lighttaskscheduler

import "context"

// TaskContainer 抽象的任务容器，需要开发者可以选择使用已有的任务容器，也可以根据实际业务实现自己的任务容器接口
type TaskContainer interface {

	// AddTask 向容器添加任务
	AddTask(ctx context.Context, task Task) (err error)

	// GetRunningTask 获取所有运行中的任务
	GetRunningTask(ctx context.Context) (tasks []Task, err error)

	// GetRunningTaskCount 获取运行中的任务数
	GetRunningTaskCount(ctx context.Context) (count int32, err error)

	// GetWaitingTask 获取等待运行中的任务
	GetWaitingTask(ctx context.Context, limit int32) (tasks []Task, err error)

	// ToRunningStatus 转移到运行中的状态
	ToRunningStatus(ctx context.Context, task *Task) (newTask *Task, err error)

	// ToStopStatus 转移到停止状态
	ToStopStatus(ctx context.Context, task *Task) (newTask *Task, err error)

	// ToDeleteStatus 转移到删除状态
	ToDeleteStatus(ctx context.Context, task *Task) (newTask *Task, err error)

	// ToFailedStatus 转移到失败状态
	ToFailedStatus(ctx context.Context, task *Task, reason error) (newTask *Task, err error)

	// ToExportStatus 转移到结果导出状态
	ToExportStatus(ctx context.Context, task *Task) (newTask *Task, err error)

	// ToSuccessStatus 转移到执行成功状态
	ToSuccessStatus(ctx context.Context, task *Task) (newTask *Task, err error)

	// UpdateRunningTaskStatus 更新执行中的任务执行进度状态
	UpdateRunningTaskStatus(ctx context.Context, task *Task, status AsyncTaskStatus) error

	// SaveData 提供一个把从任务执行器获取的任务执行的结果进行存储的机会
	// data 协议保持和 TaskActuator.GetOutput 一样
	SaveData(ctx context.Context, task *Task, data interface{}) (err error)
}
