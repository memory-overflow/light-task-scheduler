package memeorycontainer

import (
	"context"

	lighttaskscheduler "github.com/memory-overflow/light-task-scheduler"
)

// redisContainer redis 作为容器，支持任务优先级，并且可以多进程，多副本共享数据
type redisContainer struct {
	MemeoryContainer
	// TODO
}

// MakeredisContainer 构造队列型任务容器
func MakeredisContainer() *redisContainer {
	// TODO
	return &redisContainer{}
}

// AddTask 添加任务
func (r *redisContainer) AddTask(ctx context.Context, task lighttaskscheduler.Task) (err error) {
	// TODO
	return nil
}

// GetRunningTask 获取运行中的任务
func (r *redisContainer) GetRunningTask(ctx context.Context) (tasks []lighttaskscheduler.Task, err error) {
	// TODO
	return tasks, err
}

// GetRunningTaskCount 获取运行中的任务数
func (r *redisContainer) GetRunningTaskCount(ctx context.Context) (count int32, err error) {
	// TODO
	return 0, nil
}

// GetWaitingTask 获取等待中的任务
func (r *redisContainer) GetWaitingTask(ctx context.Context, limit int32) (tasks []lighttaskscheduler.Task, err error) {
	// TODO
	return tasks, nil
}

// ToRunningStatus 转移到运行中的状态
func (r *redisContainer) ToRunningStatus(ctx context.Context, task *lighttaskscheduler.Task) (
	newTask *lighttaskscheduler.Task, err error) {
	// TODO
	return task, nil
}

// ToExportStatus 转移到停止状态
func (r *redisContainer) ToStopStatus(ctx context.Context, task *lighttaskscheduler.Task) (
	newTask *lighttaskscheduler.Task, err error) {
	// TODO
	return task, nil
}

// ToExportStatus 转移到删除状态
func (r *redisContainer) ToDeleteStatus(ctx context.Context, task *lighttaskscheduler.Task) (
	newTask *lighttaskscheduler.Task, err error) {
	// TODO
	return task, nil
}

// ToFailedStatus 转移到失败状态
func (r *redisContainer) ToFailedStatus(ctx context.Context, task *lighttaskscheduler.Task, reason error) (
	newTask *lighttaskscheduler.Task, err error) {
	// TODO
	return
}

// ToExportStatus 转移到数据导出状态
func (r *redisContainer) ToExportStatus(ctx context.Context, task *lighttaskscheduler.Task) (
	newTask *lighttaskscheduler.Task, err error) {
	// TODO
	return task, nil
}

// ToSuccessStatus 转移到执行成功状态
func (r *redisContainer) ToSuccessStatus(ctx context.Context, task *lighttaskscheduler.Task) (
	newTask *lighttaskscheduler.Task, err error) {
	// TODO
	return task, nil
}

// UpdateRunningTaskStatus 更新执行中的任务状态
func (r *redisContainer) UpdateRunningTaskStatus(ctx context.Context,
	task *lighttaskscheduler.Task, status lighttaskscheduler.AsyncTaskStatus) error {
	// TODO
	return nil
}
