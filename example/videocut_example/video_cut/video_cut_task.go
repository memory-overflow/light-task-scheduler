package videocut

import (
	"time"

	framework "github.com/memory-overflow/light-task-scheduler"
	"gorm.io/gorm"
)

// VideoCutTask 视频裁剪任务结构
type VideoCutTask struct {
	TaskId       string               `gorm:"primary_key;type:varchar(1024);index:task_id_idx"` // 任务系统的唯一任务 Id
	InputVideo   string               `gorm:"type:varchar(4096);default:''"`                    // 输入视频
	OutputVideo  string               `gorm:"type:varchar(4096);default:''"`                    // 输出视频
	CutStartTime float32              `gorm:"default:0"`                                        // 裁剪开始的秒数
	CutEndTime   float32              `gorm:"default:0"`                                        // 裁剪结束的秒数
	Status       framework.TaskStatus `gorm:"type:tinyint"`                                     // 任务状态
	WorkTaskId   string               `gorm:"type:varchar(1024);default:''"`                    // 微服务引擎的任务 id
	FailedReason string               `gorm:"type:varchar(4096);default:''"`
	CreatedAt    time.Time            `gorm:"column:create_time"`
	UpdatedAt    time.Time            `gorm:"column:update_time"`
	DeletedAt    gorm.DeletedAt       `gorm:"default:NULL;column:delete_time"`
	StartAt      *time.Time           `gorm:"default:NULL;column:start_time"` // 任务开始时间
	EndAt        *time.Time           `gorm:"default:NULL;column:end_time"`   // 任务结束时间
	AttemptsTime int                  `gorm:"default:0"`                      // 重试次数
}

// TableName 更改数据库表名
func (VideoCutTask) TableName() string {
	return "task"
}
