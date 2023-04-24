package persistcontainer

import (
	"context"

	lighttaskscheduler "github.com/memory-overflow/light-task-scheduler"
)

// exampleSQLContainer sql db 作为容器，可以根据本实现调整自己的数据表
type exampleSQLContainer struct {
	PersistContainer
	// TODO
}

// MakeexampleSQLContainer 构造队列型任务容器
func MakeexampleSQLContainer() *exampleSQLContainer {
	// TODO
	return &exampleSQLContainer{}
}

// AddTask 添加任务
func (e *exampleSQLContainer) AddTask(ctx context.Context, task lighttaskscheduler.Task) (err error) {
	// TODO
	return nil
}

// GetRunningTask 获取运行中的任务
func (e *exampleSQLContainer) GetRunningTask(ctx context.Context) (tasks []lighttaskscheduler.Task, err error) {
	// TODO
	return tasks, err
}

// GetRunningTaskCount 获取运行中的任务数
func (e *exampleSQLContainer) GetRunningTaskCount(ctx context.Context) (count int32, err error) {
	// TODO
	return 0, nil
}

// GetWaitingTask 获取等待中的任务
func (e *exampleSQLContainer) GetWaitingTask(ctx context.Context, limit int32) (tasks []lighttaskscheduler.Task, err error) {
	// TODO
	return tasks, nil
}

// ToRunningStatus 转移到运行中的状态
func (e *exampleSQLContainer) ToRunningStatus(ctx context.Context, task *lighttaskscheduler.Task) (
	newTask *lighttaskscheduler.Task, err error) {
	// TODO
	return task, nil
}

// ToExportStatus 转移到停止状态
func (e *exampleSQLContainer) ToStopStatus(ctx context.Context, task *lighttaskscheduler.Task) (
	newTask *lighttaskscheduler.Task, err error) {
	// TODO
	return task, nil
}

// ToExportStatus 转移到删除状态
func (e *exampleSQLContainer) ToDeleteStatus(ctx context.Context, task *lighttaskscheduler.Task) (
	newTask *lighttaskscheduler.Task, err error) {
	// TODO
	return task, nil
}

// ToFailedStatus 转移到失败状态
func (e *exampleSQLContainer) ToFailedStatus(ctx context.Context, task *lighttaskscheduler.Task, reason error) (
	newTask *lighttaskscheduler.Task, err error) {
	// TODO
	return
}

// ToExportStatus 转移到数据导出状态
func (e *exampleSQLContainer) ToExportStatus(ctx context.Context, task *lighttaskscheduler.Task) (
	newTask *lighttaskscheduler.Task, err error) {
	// TODO
	return task, nil
}

// ToSuccessStatus 转移到执行成功状态
func (e *exampleSQLContainer) ToSuccessStatus(ctx context.Context, task *lighttaskscheduler.Task) (
	newTask *lighttaskscheduler.Task, err error) {
	// TODO
	return task, nil
}

// UpdateRunningTaskStatus 更新执行中的任务状态
func (e *exampleSQLContainer) UpdateRunningTaskStatus(ctx context.Context,
	task *lighttaskscheduler.Task, status lighttaskscheduler.AsyncTaskStatus) error {
	// TODO
	return nil
}
