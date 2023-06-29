package lighttaskscheduler

import "context"

// TaskdataPersistencer 任务数据的可持久化接口
type TaskdataPersistencer interface {
	// DataPersistence 定义如何把通过执行器 GetOutput 的数据进行此久化存储
	// data 的协议保持和 TaskActuator.GetOutput 一样
	DataPersistence(ctx context.Context, task *Task, data interface{}) (err error)

	// GetPersistenceData 查询任务持久化结果
	GetPersistenceData(ctx context.Context, task *Task) (data interface{}, err error)

	// DeletePersistenceData 删除任务的此久化结果
	DeletePersistenceData(ctx context.Context, task *Task) (err error)
}
