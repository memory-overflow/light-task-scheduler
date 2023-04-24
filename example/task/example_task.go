package task

import "time"

// ExampleTask 任务结构
type ExampleTask struct {
	TaskId    uint32
	StartTime time.Time
	A, B      int32
}
