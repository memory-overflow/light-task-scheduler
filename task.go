package lighttaskscheduler

import "time"

// TaskStatus 任务状态
type TaskStatus int32

const (
	TASK_STATUS_INVALID   TaskStatus = 0
	TASK_STATUS_UNSTART   TaskStatus = 1
	TASK_STATUS_WAITING   TaskStatus = 2
	TASK_STATUS_RUNNING   TaskStatus = 3
	TASK_STATUS_SUCCESS   TaskStatus = 4
	TASK_STATUS_FAILED    TaskStatus = 5
	TASK_STATUS_STOPED    TaskStatus = 6
	TASK_STATUS_DELETE    TaskStatus = 7
	TASK_STATUS_EXPORTING TaskStatus = 8
)

// Task 通用的任务结构
type Task struct {
	// 该任务的唯一标识id，创建任务的时候赋予
	TaskId string
	// 任务优先级, 创建任务的时候可选
	TaskPriority int
	// 任务对象，创建任务的时候赋予
	TaskItem interface{}

	TaskStartTime time.Time // 框架赋予值
	TaskEnbTime   time.Time // 框架赋予值
	// 任务状态，任务容器负责赋予值
	TaskStatus TaskStatus
	// 任务容器负责赋予值
	FailedReason string
	// 任务已经重试的次数，任务容器负责赋予值
	TaskAttemptsTime int32
}

// AsyncTaskStatus 异步任务状态
type AsyncTaskStatus struct {
	TaskStatus   TaskStatus
	FailedReason error
	Progress     interface{}
}
