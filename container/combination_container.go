package container

import (
	"context"
	"fmt"

	lighttaskscheduler "github.com/memory-overflow/light-task-scheduler"
	memeorycontainer "github.com/memory-overflow/light-task-scheduler/container/memory_container"
	persistcontainer "github.com/memory-overflow/light-task-scheduler/container/persist_container"
)

// combinationContainer 组合 MemeoryContainer 和 Persistcontainer 的容器
// 整合两种容器的优点，既能够通过内存实现快写快读，又能够通过DB实现可持久化
type combinationContainer struct {
	memeoryContainer memeorycontainer.MemeoryContainer
	persistContainer persistcontainer.PersistContainer
}

// MakeCombinationContainer 构造队列型任务容器
func MakeCombinationContainer(memeoryContainer memeorycontainer.MemeoryContainer,
	persistContainer persistcontainer.PersistContainer) *combinationContainer {
	com := &combinationContainer{
		memeoryContainer: memeoryContainer,
		persistContainer: persistContainer,
	}
	ctx := context.Background()
	if tasks, err := persistContainer.GetRunningTask(ctx); err == nil {
		for _, t := range tasks {
			memeoryContainer.AddRunningTask(ctx, t)
		}
	}
	for {
		batchSize := 1000
		if tasks, err := persistContainer.GetWaitingTask(ctx, int32(batchSize)); err == nil {
			for _, t := range tasks {
				memeoryContainer.AddTask(ctx, t)
			}
			if len(tasks) < batchSize {
				break
			}
		} else {
			break
		}
	}
	return com
}

// AddTask 添加任务
func (c *combinationContainer) AddTask(ctx context.Context, task lighttaskscheduler.Task) (err error) {
	if err = c.memeoryContainer.AddTask(ctx, task); err != nil {
		return fmt.Errorf("memeoryContainer AddTask error: %v", err)
	}
	defer func() {
		if err != nil {
			c.memeoryContainer.ToDeleteStatus(ctx, &task)
		}
	}()
	if err = c.persistContainer.AddTask(ctx, task); err != nil {
		return fmt.Errorf("persistContainer AddTask error: %v", err)
	}
	return nil
}

// GetRunningTask 获取运行中的任务
func (c *combinationContainer) GetRunningTask(ctx context.Context) (tasks []lighttaskscheduler.Task, err error) {
	return c.memeoryContainer.GetRunningTask(ctx)
}

// GetRunningTaskCount 获取运行中的任务数
func (c *combinationContainer) GetRunningTaskCount(ctx context.Context) (count int32, err error) {
	return c.memeoryContainer.GetRunningTaskCount(ctx)
}

// GetWaitingTask 获取等待中的任务
func (c *combinationContainer) GetWaitingTask(ctx context.Context, limit int32) (tasks []lighttaskscheduler.Task, err error) {
	return c.memeoryContainer.GetWaitingTask(ctx, limit)
}

// ToRunningStatus 转移到运行中的状态
func (c *combinationContainer) ToRunningStatus(ctx context.Context, task *lighttaskscheduler.Task) (
	newTask *lighttaskscheduler.Task, err error) {
	if newTask, err = c.persistContainer.ToRunningStatus(ctx, task); err != nil {
		return newTask, fmt.Errorf("persistContainer ToRunningStatus error: %v", err)
	}
	if newTask, err = c.memeoryContainer.ToRunningStatus(ctx, task); err != nil {
		return newTask, fmt.Errorf("memeoryContainer ToRunningStatus error: %v", err)
	}
	return newTask, nil
}

// ToExportStatus 转移到停止状态
func (c *combinationContainer) ToStopStatus(ctx context.Context, task *lighttaskscheduler.Task) (
	newTask *lighttaskscheduler.Task, err error) {

	if newTask, err = c.persistContainer.ToStopStatus(ctx, task); err != nil {
		return newTask, fmt.Errorf("persistContainer ToStopStatus error: %v", err)
	}
	if newTask, err = c.memeoryContainer.ToStopStatus(ctx, task); err != nil {
		return newTask, fmt.Errorf("memeoryContainer ToRunningStatus error: %v", err)
	}
	return newTask, nil
}

// ToExportStatus 转移到删除状态
func (c *combinationContainer) ToDeleteStatus(ctx context.Context, task *lighttaskscheduler.Task) (
	newTask *lighttaskscheduler.Task, err error) {
	if newTask, err = c.persistContainer.ToDeleteStatus(ctx, task); err != nil {
		return newTask, fmt.Errorf("persistContainer ToStopStatus error: %v", err)
	}
	if newTask, err = c.memeoryContainer.ToDeleteStatus(ctx, task); err != nil {
		return newTask, fmt.Errorf("memeoryContainer ToRunningStatus error: %v", err)
	}
	return newTask, nil
}

// ToFailedStatus 转移到失败状态
func (c *combinationContainer) ToFailedStatus(ctx context.Context, task *lighttaskscheduler.Task, reason error) (
	newTask *lighttaskscheduler.Task, err error) {
	if newTask, err = c.persistContainer.ToFailedStatus(ctx, task, reason); err != nil {
		return newTask, fmt.Errorf("persistContainer ToFailedStatus error: %v", err)
	}
	if newTask, err = c.memeoryContainer.ToFailedStatus(ctx, task, reason); err != nil {
		return newTask, fmt.Errorf("memeoryContainer ToFailedStatus error: %v", err)
	}
	return newTask, nil
}

// ToExportStatus 转移到数据导出状态
func (c *combinationContainer) ToExportStatus(ctx context.Context, task *lighttaskscheduler.Task) (
	newTask *lighttaskscheduler.Task, err error) {
	if newTask, err = c.persistContainer.ToExportStatus(ctx, task); err != nil {
		return newTask, fmt.Errorf("persistContainer ToExportStatus error: %v", err)
	}
	if newTask, err = c.memeoryContainer.ToExportStatus(ctx, task); err != nil {
		return newTask, fmt.Errorf("memeoryContainer ToExportStatus error: %v", err)
	}
	return newTask, nil
}

// ToSuccessStatus 转移到执行成功状态
func (c *combinationContainer) ToSuccessStatus(ctx context.Context, task *lighttaskscheduler.Task) (
	newTask *lighttaskscheduler.Task, err error) {
	if newTask, err = c.persistContainer.ToSuccessStatus(ctx, task); err != nil {
		return newTask, fmt.Errorf("persistContainer ToSuccessStatus error: %v", err)
	}
	if newTask, err = c.memeoryContainer.ToSuccessStatus(ctx, task); err != nil {
		return newTask, fmt.Errorf("memeoryContainer ToSuccessStatus error: %v", err)
	}
	return newTask, nil
}

// UpdateRunningTaskStatus 更新执行中的任务状态
func (c *combinationContainer) UpdateRunningTaskStatus(ctx context.Context,
	task *lighttaskscheduler.Task, status lighttaskscheduler.AsyncTaskStatus) error {
	if err := c.persistContainer.UpdateRunningTaskStatus(ctx, task, status); err != nil {
		return fmt.Errorf("persistContainer UpdateRunningTaskStatus error: %v", err)
	}
	if err := c.memeoryContainer.UpdateRunningTaskStatus(ctx, task, status); err != nil {
		return fmt.Errorf("memeoryContainer UpdateRunningTaskStatus error: %v", err)
	}
	return nil
}

// SaveData 导出任务输出，自行处理任务结果
func (c *combinationContainer) SaveData(ctx context.Context, ftask *lighttaskscheduler.Task,
	data interface{}) error {
	if err := c.memeoryContainer.SaveData(ctx, ftask, data); err != nil {
		return err
	}
	if err := c.persistContainer.SaveData(ctx, ftask, data); err != nil {
		return err
	}
	return nil
}
