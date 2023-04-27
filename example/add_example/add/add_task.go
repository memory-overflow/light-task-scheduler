package addexample

import "time"

// AddTask add 任务结构
type AddTask struct {
	StartTime time.Time
	A, B      int32
}
