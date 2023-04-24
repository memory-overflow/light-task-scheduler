package memeorycontainer

import (
	"context"

	lighttaskscheduler "github.com/memory-overflow/light-task-scheduler"
)

// MemeoryContainer 内存型任务容器，优先：快读快写，缺点：不可持久化，
type MemeoryContainer interface {
	lighttaskscheduler.TaskContainer

	// AddRunningTask 向容器添加正在运行中的任务
	// 对于某些可持久化任务，调度器如果因为某些原因退出，需要从 db 中恢复状态，这个接口用来向容器中添加恢复前还在执行中的任务
	AddRunningTask(ctx context.Context, task lighttaskscheduler.Task) (err error)
}
