package videocut

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	framework "github.com/memory-overflow/light-task-scheduler"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormlogger "gorm.io/gorm/logger"
)

// videoCutSqlContainer sql db 作为容器，可以根据本实现调整自己的数据表
type videoCutSqlContainer struct {
	db *gorm.DB // 数据库连接
}

// MakeVideoCutSqlContainer 构造数据库容器
func MakeVideoCutSqlContainer(dbHost, dbPort, dbUser, dbPass, dbName string) (
	sqlContainer *videoCutSqlContainer, err error) {

	// 获取 gorm 数据库连接实例
	connStr := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8&parseTime=True&loc=Local&interpolateParams=true",
		dbUser, dbPass, dbHost, dbPort, dbName)
	newLogger := gormlogger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		gormlogger.Config{
			SlowThreshold: 5 * time.Second, // Slow SQL threshold
			LogLevel:      gormlogger.Warn, // Log level
			Colorful:      true,            // enable color
		},
	)
	db, err := gorm.Open(mysql.Open(connStr), &gorm.Config{
		CreateBatchSize: 500,
		Logger:          newLogger,
	})
	if err != nil {
		return nil, errors.New("Connect db failed, conn str: " + connStr + ", error: " + err.Error())
	}
	if db == nil {
		return nil, errors.New("Connect db failed, conn str: " + connStr + ", unknow error")
	}
	sql, err := db.DB()
	if err != nil {
		return nil, errors.New("Connect db failed, conn str: " + connStr + ", unknow error")
	}
	// 配置 db 限制
	sql.SetMaxOpenConns(200)
	sql.SetMaxIdleConns(200)
	sql.SetConnMaxLifetime(time.Minute)
	sql.SetConnMaxIdleTime(time.Minute)
	// db = db.Debug()
	// 自动 migrate task 表
	err = db.Set("gorm:table_options", "ENGINE=InnoDB DEFAULT CHARSET=utf8").AutoMigrate(&VideoCutTask{})
	if err != nil {
		return nil, errors.New("migrate table failed: " + err.Error())
	}
	return &videoCutSqlContainer{db: db}, nil
}

// AddTask 添加任务
func (e *videoCutSqlContainer) AddTask(ctx context.Context, ftask framework.Task) (err error) {

	task, ok := ftask.TaskItem.(VideoCutTask)
	if !ok {
		return fmt.Errorf("TaskItem not be set to AddTask")
	}
	db := e.db
	t := time.Now()
	task.Status = framework.TASK_STATUS_WAITING
	task.StartAt = &t
	task.EndAt = nil

	if err = db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "task_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"status", "start_time", "end_time"}),
	}).Create(&task).Error; err != nil {
		err = fmt.Errorf("db create error: %v", err)
		log.Println(err)
		return err
	}
	return nil
}

// GetRunningTask 获取运行中的任务
func (e *videoCutSqlContainer) GetRunningTask(ctx context.Context) (tasks []framework.Task, err error) {
	db := e.db
	taskRecords := []VideoCutTask{}
	if err = db.Where("status = ?", framework.TASK_STATUS_RUNNING).Find(&taskRecords).Error; err != nil {
		err = fmt.Errorf("db find error: %v", err)
		log.Panicln(err)
		return nil, err
	}
	for _, taskRecord := range taskRecords {
		task := framework.Task{
			TaskId:     taskRecord.TaskId,
			TaskItem:   taskRecord,
			TaskStatus: taskRecord.Status,
		}
		if taskRecord.StartAt != nil {
			task.TaskStartTime = *taskRecord.StartAt
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// GetRunningTaskCount 获取运行中的任务数
func (e *videoCutSqlContainer) GetRunningTaskCount(ctx context.Context) (count int32, err error) {
	db := e.db
	var tot int64
	if err = db.Model(&VideoCutTask{}).Where("status = ?",
		framework.TASK_STATUS_RUNNING).Count(&tot).Error; err != nil {
		err = fmt.Errorf("db count error: %v", err)
		log.Println(err)
		return 0, err
	}
	return int32(tot), nil
}

// GetWaitingTask 获取等待中的任务
func (e *videoCutSqlContainer) GetWaitingTask(ctx context.Context, limit int32) (tasks []framework.Task, err error) {
	db := e.db
	taskRecords := []VideoCutTask{}
	if err = db.Where("status = ?", framework.TASK_STATUS_WAITING).
		Order("start_time asc"). // 按照开始时间优先运行
		Limit(int(limit)).Find(&taskRecords).Error; err != nil {
		err = fmt.Errorf("db create error: %v", err)
		log.Println(err)
		return nil, err
	}
	for _, taskRecord := range taskRecords {
		task := framework.Task{
			TaskId:     taskRecord.TaskId,
			TaskItem:   taskRecord,
			TaskStatus: taskRecord.Status,
		}
		if taskRecord.StartAt != nil {
			task.TaskStartTime = *taskRecord.StartAt
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// ToRunningStatus 转移到运行中的状态
func (e *videoCutSqlContainer) ToRunningStatus(ctx context.Context, ftask *framework.Task) (
	newTask *framework.Task, err error) {
	defer func() {
		if err != nil {
			log.Println("ToRunningStatus: ", err)
		}
	}()
	task, ok := ftask.TaskItem.(VideoCutTask)
	if !ok {
		return ftask, fmt.Errorf("TaskItem not be set to AddTask")
	}
	db := e.db
	t := time.Now()
	sql := db.Model(&VideoCutTask{}).Where("task_id = ? and status = ?", ftask.TaskId, ftask.TaskStatus).
		Updates(map[string]interface{}{
			"status":       framework.TASK_STATUS_RUNNING,
			"start_time":   t,
			"work_task_id": task.WorkTaskId,
		})
	if sql.Error != nil {
		return ftask, fmt.Errorf("db update error: %v", sql.Error)
	}
	if sql.RowsAffected == 0 {
		return ftask, fmt.Errorf("task %s not found, may status has been changed", task.TaskId)
	}
	task.Status, ftask.TaskStatus = framework.TASK_STATUS_RUNNING, framework.TASK_STATUS_RUNNING
	task.StartAt, ftask.TaskStartTime = &t, t
	ftask.TaskItem = task
	return ftask, nil
}

// ToExportStatus 转移到停止状态
func (e *videoCutSqlContainer) ToStopStatus(ctx context.Context, ftask *framework.Task) (
	newTask *framework.Task, err error) {
	defer func() {
		if err != nil {
			log.Println("ToStopStatus: ", err)
		}
	}()

	task, ok := ftask.TaskItem.(VideoCutTask)
	if !ok {
		return ftask, fmt.Errorf("TaskItem not be set to AddTask")
	}
	db := e.db
	sql := db.Model(&VideoCutTask{}).Where("task_id = ? and status = ?", ftask.TaskId, ftask.TaskStatus).
		Update("status", framework.TASK_STATUS_STOPED)
	if sql.Error != nil {
		return ftask, fmt.Errorf("db update error: %v", sql.Error)
	}
	if sql.RowsAffected == 0 {
		return ftask, fmt.Errorf("task %s not found, may status has been changed", task.TaskId)
	}
	task.Status, ftask.TaskStatus = framework.TASK_STATUS_STOPED, framework.TASK_STATUS_STOPED
	ftask.TaskItem = task
	return ftask, nil
}

// ToExportStatus 转移到删除状态
func (e *videoCutSqlContainer) ToDeleteStatus(ctx context.Context, ftask *framework.Task) (
	newTask *framework.Task, err error) {
	defer func() {
		if err != nil {
			log.Println("ToDeleteStatus: ", err)
		}
	}()
	task, ok := ftask.TaskItem.(VideoCutTask)
	if !ok {
		return ftask, fmt.Errorf("TaskItem not be set to AddTask")
	}
	db := e.db
	sql := db.Model(&VideoCutTask{}).Delete("task_id = ? and status = ?", ftask.TaskId, ftask.TaskStatus)
	if sql.Error != nil {
		return ftask, fmt.Errorf("db delete error: %v", sql.Error)
	}
	if sql.RowsAffected == 0 {
		return ftask, fmt.Errorf("task %s not found, may status has been changed", task.TaskId)
	}
	task.Status, ftask.TaskStatus = framework.TASK_STATUS_DELETE, framework.TASK_STATUS_DELETE
	ftask.TaskItem = task
	return ftask, nil
}

// ToFailedStatus 转移到失败状态
func (e *videoCutSqlContainer) ToFailedStatus(ctx context.Context, ftask *framework.Task, reason error) (
	newTask *framework.Task, err error) {
	defer func() {
		if err != nil {
			log.Println("ToFailedStatus: ", err)
		}
	}()
	task, ok := ftask.TaskItem.(VideoCutTask)
	if !ok {
		return ftask, fmt.Errorf("TaskItem not be set to AddTask")
	}
	db := e.db
	t := time.Now()
	sql := db.Model(&VideoCutTask{}).Where("task_id = ? and status = ?", ftask.TaskId, ftask.TaskStatus).
		Updates(map[string]interface{}{
			"status":        framework.TASK_STATUS_FAILED,
			"end_time":      t,
			"failed_reason": reason.Error(),
		})
	if sql.Error != nil {
		return ftask, fmt.Errorf("db update error: %v", sql.Error)
	}
	if sql.RowsAffected == 0 {
		return ftask, fmt.Errorf("task %s not found, may status has been changed", task.TaskId)
	}
	task.Status, ftask.TaskStatus = framework.TASK_STATUS_FAILED, framework.TASK_STATUS_FAILED
	task.FailedReason, ftask.FailedReason = reason.Error(), reason.Error()
	task.EndAt, ftask.TaskEnbTime = &t, t
	ftask.TaskItem = task
	return ftask, nil
}

// ToExportStatus 转移到数据导出状态
func (e *videoCutSqlContainer) ToExportStatus(ctx context.Context, ftask *framework.Task) (
	newTask *framework.Task, err error) {
	defer func() {
		if err != nil {
			log.Println("ToExportStatus: ", err)
		}
	}()
	task, ok := ftask.TaskItem.(VideoCutTask)
	if !ok {
		return ftask, fmt.Errorf("TaskItem not be set to AddTask")
	}
	db := e.db
	sql := db.Model(&VideoCutTask{}).Where("task_id = ? and status = ?", ftask.TaskId, ftask.TaskStatus).
		Update("status", framework.TASK_STATUS_EXPORTING)
	if sql.Error != nil {
		return ftask, fmt.Errorf("db update error: %v", sql.Error)
	}
	if sql.RowsAffected == 0 {
		return ftask, fmt.Errorf("task %s not found, may status has been changed", task.TaskId)
	}
	task.Status, ftask.TaskStatus = framework.TASK_STATUS_EXPORTING, framework.TASK_STATUS_EXPORTING
	ftask.TaskItem = task
	return ftask, nil
}

// ToSuccessStatus 转移到执行成功状态
func (e *videoCutSqlContainer) ToSuccessStatus(ctx context.Context, ftask *framework.Task) (
	newTask *framework.Task, err error) {
	defer func() {
		if err != nil {
			log.Println("ToSuccessStatus: ", err)
		}
	}()
	task, ok := ftask.TaskItem.(VideoCutTask)
	if !ok {
		return ftask, fmt.Errorf("TaskItem not be set to AddTask")
	}
	db := e.db
	t := time.Now()
	sql := db.Model(&VideoCutTask{}).Where("task_id = ? and status = ?", ftask.TaskId, ftask.TaskStatus).
		Updates(map[string]interface{}{
			"status":   framework.TASK_STATUS_SUCCESS,
			"end_time": t,
		})
	if sql.Error != nil {
		return ftask, fmt.Errorf("db update error: %v", sql.Error)
	}
	if sql.RowsAffected == 0 {
		return ftask, fmt.Errorf("task %s not found, may status has been changed", task.TaskId)
	}
	task.Status, ftask.TaskStatus = framework.TASK_STATUS_SUCCESS, framework.TASK_STATUS_SUCCESS
	task.EndAt, ftask.TaskEnbTime = &t, t
	ftask.TaskItem = task
	return ftask, nil
}

// UpdateRunningTaskStatus 更新执行中的任务状态
func (e *videoCutSqlContainer) UpdateRunningTaskStatus(ctx context.Context,
	ftask *framework.Task, status framework.AsyncTaskStatus) error {
	return nil
}

// SaveData 提供一个把从任务执行器获取的任务执行的结果进行存储的机会
// data 协议保持和 TaskActuator.GetOutput 一样, 一个 string 表示结果路径
func (e *videoCutSqlContainer) SaveData(ctx context.Context, ftask *framework.Task,
	data interface{}) (err error) {
	defer func() {
		if err != nil {
			log.Println("SaveData:", err)
		}
	}()

	task, ok := ftask.TaskItem.(VideoCutTask)
	if !ok {
		return fmt.Errorf("TaskItem not be set to AddTask")
	}
	db := e.db
	outputVideo, ok := data.(string)
	if !ok {
		return fmt.Errorf("data not be set to string")
	}
	sql := db.Model(&VideoCutTask{}).Where("task_id = ?", ftask.TaskId).
		Updates(map[string]interface{}{
			"output_video": outputVideo,
		})
	if sql.Error != nil {
		return fmt.Errorf("db update error: %v", sql.Error)
	}
	if sql.RowsAffected == 0 {
		return fmt.Errorf("task %s not found, may status has been changed", task.TaskId)
	}
	task.OutputVideo = outputVideo
	ftask.TaskItem = task
	return nil
}

// 下面的方法用来做其他的业务查询

// GetTaskInfoSet 分页拉取任务列表接口
func (e *videoCutSqlContainer) GetTasks(ctx context.Context, pageNumber, pageSize int) (
	tasks []VideoCutTask, count int, err error) {
	db := e.db
	tempDb := db.Model(VideoCutTask{}).Where("delete_time is null")
	orderStr := "create_time DESC"
	var total int64
	tempDb.Count(&total)
	count = int(total)
	tempDb = tempDb.Order(orderStr).Offset((pageNumber - 1) * pageSize).Limit(pageSize)
	if e := tempDb.Find(&tasks).Error; e != nil {
		return nil, 0, e
	}
	return tasks, count, nil
}

// CreateTask 创建任务
func (e *videoCutSqlContainer) CreateTask(ctx context.Context, task VideoCutTask) (err error) {
	db := e.db
	if err = db.Model(&task).Create(&task).Error; err != nil {
		err = fmt.Errorf("db create error: %v", err)
		log.Println(err)
		return err
	}
	return nil
}

// GetTask 根据 taskId 查询任务
func (e *videoCutSqlContainer) GetTask(ctx context.Context, taskId string) (
	task *VideoCutTask, err error) {
	db := e.db
	tempTask := VideoCutTask{}
	if err = db.Model(&tempTask).Where("task_id = ?", taskId).First(&tempTask).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("不存在的任务")
		} else {
			err = fmt.Errorf("db first error: %v", err)
			log.Println(err)
		}
		return nil, err
	}
	return &tempTask, nil
}
