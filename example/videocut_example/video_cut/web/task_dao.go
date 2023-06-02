package web

// import (
// 	"context"
// 	"errors"
// 	"fmt"
// 	"math"
// 	"os"
// 	"strconv"
// 	"time"

// 	"git.code.oa.com/trpc-go/trpc-go/log"
// 	"git.code.oa.com/yt-media-ai-videounderstanding/yt-ai-media-middle-platform-api/protobuf-spec/apicommon"
// 	applog "git.code.oa.com/yt-media-ai-videounderstanding/yt-ai-media-middle-platform/common/log"
// 	"git.code.oa.com/yt-media-ai-videounderstanding/yt-ai-media-middle-platform/models"
// 	videocut "github.com/memory-overflow/light-task-scheduler/example/videocut_example/video_cut"
// 	"gorm.io/gorm"
// )

// // TaskDao ...
// type TaskDao struct {
// }

// // GetTaskStatus 获取任务的状态 返回 flowID 和 任务状态
// func (t TaskDao) GetTaskStatus(ctx context.Context, taskID int) (
// 	flowID, status string, err error) {
// 	db := config.GetGlobalDB().WithContext(ctx)
// 	var task models.Task
// 	if e := db.Where("task_id = ?", taskID).First(&task).Error; e != nil {
// 		if errors.Is(e, gorm.ErrRecordNotFound) {
// 			return "", models.TaskStatusUnknown, errors.New("没有该任务")
// 		}
// 		return "", models.TaskStatusUnknown, e
// 	}
// 	if task.FlowID == nil {
// 		return "", task.Status, nil
// 	}
// 	return *task.FlowID, task.Status, nil
// }

// // GetTaskInfo 获取任务信息
// func (t TaskDao) GetTaskInfo(ctx context.Context, taskID int) (
// 	task *models.Task, info *apicommon.TaskInfo, err error) {
// 	info = new(apicommon.TaskInfo)
// 	info.TaskID = int32(taskID)
// 	db := config.GetGlobalDB().WithContext(ctx)
// 	if e := db.Where("task_id = ?", taskID).First(&task).Error; e != nil {
// 		if errors.Is(e, gorm.ErrRecordNotFound) {
// 			return task, nil, errors.New("没有该任务")
// 		}
// 		return task, nil, e
// 	}
// 	*info, err = t.GetTaskInfoByTask(ctx, task)
// 	return task, info, err
// }

// // GetTask 获取任务对象
// func (t TaskDao) GetTask(ctx context.Context, taskID int) (task *models.Task, err error) {
// 	db := config.GetGlobalDB().WithContext(ctx)
// 	if e := db.Where("task_id = ? and delete_time is null", taskID).First(&task).Error; e != nil {
// 		if e == gorm.ErrRecordNotFound {
// 			return nil, errors.New("没有该任务")
// 		} else {
// 			return nil, errors.New("DB query failed, " + e.Error())
// 		}
// 	}
// 	return task, nil
// }

// // GetExpiredTasksWithLimit 获取过期任务
// func (t TaskDao) GetExpiredTasksWithLimit(ctx context.Context, days int, limit int) (
// 	tasks []models.Task, err error) {
// 	db := config.GetGlobalDB().WithContext(ctx)
// 	db.Where("create_time < ?", time.Now().AddDate(0, 0, -days)).Limit(limit).Find(&tasks)
// 	return tasks, nil
// }

// // GetAllRunningTasks 获取所有分析中的任务
// func (t TaskDao) GetAllRunningTasks(ctx context.Context) (tasks []models.Task, err error) {

// 	db := config.GetGlobalDB().WithContext(ctx)
// 	err = db.Model(&models.Task{}).Where(
// 		"task.status = ? and task.delete_time is null and task.flow_id is not null",
// 		models.TaskStatusRunning).Find(&tasks).Error
// 	return tasks, err
// }

// // TaskMediaInfo 包含媒体的任务信息
// type TaskMediaInfo struct {
// 	models.Task
// 	MediaName      string
// 	MediaTag       string
// 	MediaSecondTag string
// 	Duration       float32
// }

// // GetTasksByDuration 获取一段时间内的任务列表，包含媒体信息
// func (t TaskDao) GetTasksByDuration(ctx context.Context, projectID, businessID uint32,
// 	appIDSet []string, startTime, endTime string) (taskMedias []TaskMediaInfo, err error) {

// 	db := config.GetGlobalDB() // 不设超时时间
// 	statment := db.Model(&models.Task{}).Select(fmt.Sprintf("%s, %s, %s, %s, %s", "ai_media_v2.task.*",
// 		"media_asset_system.media_asset.media_name", "media_asset_system.media_asset.media_tag",
// 		"media_asset_system.media_asset.media_second_tag",
// 		"json_extract(media_asset_system.media_asset.metadata, '$.duration') as duration")).
// 		Where("ai_media_v2.task.ti_project_id = ? and ai_media_v2.task.ti_business_id = ?",
// 			projectID, businessID)
// 	if len(appIDSet) > 0 {
// 		statment = statment.Where("ai_media_v2.task.app_id IN (?)", appIDSet)
// 	}
// 	if startTime != "" {
// 		statment = statment.Where("ai_media_v2.task.create_time >= ?", startTime)
// 	}
// 	if endTime != "" {
// 		statment = statment.Where("ai_media_v2.task.create_time <= ?", endTime)
// 	}
// 	join := fmt.Sprintf("left join media_asset_system.media_asset on %s = %s",
// 		"media_asset_system.media_asset.media_id",
// 		"ai_media_v2.task.media_id")
// 	statment = statment.Joins(join).Order("ai_media_v2.task.create_time DESC")
// 	if err := statment.Scan(&taskMedias).Error; err != nil {
// 		return nil, err
// 	}
// 	return taskMedias, nil
// }

// type taskAndTransCodingRecord struct {
// 	models.Task
// 	TaskCreateTime   time.Time // 有冲突的字段单独定义
// 	TaskDeleteTime   *time.Time
// 	TaskUpdateTime   time.Time
// 	TaskMediaID      *int
// 	TaskReason       string
// 	TaskStatus       string
// 	TaskUserName     string
// 	TaskProject      uint32
// 	TaskTIBusinessID uint32
// 	TaskPriority     int

// 	models.TransCodingRecord
// 	RecordCreateTime   time.Time // 有冲突的字段单独定义
// 	RecordDeleteTime   *time.Time
// 	RecordUpdateTime   time.Time
// 	RecordMediaID      int
// 	RecordReason       string
// 	RecordStatus       string
// 	RecordUserName     string
// 	RecordProject      uint32
// 	RecordTIBusinessID uint32
// 	RecordPriority     int
// }

// func (tp *taskAndTransCodingRecord) reBuildAfterQuery() {
// 	tp.Task.CreatedAt = tp.TaskCreateTime
// 	tp.Task.DeletedAt = tp.TaskDeleteTime
// 	tp.Task.UpdatedAt = tp.TaskUpdateTime
// 	tp.Task.MediaID = tp.TaskMediaID
// 	tp.Task.Reason = tp.TaskReason
// 	tp.Task.Status = tp.TaskStatus
// 	tp.Task.UserName = tp.TaskUserName
// 	tp.Task.TIProjectID = tp.TaskProject
// 	tp.Task.TIBusinessID = tp.TaskTIBusinessID
// 	tp.Task.Priority = tp.TaskPriority

// 	tp.TransCodingRecord.CreatedAt = tp.RecordCreateTime
// 	tp.TransCodingRecord.DeletedAt = tp.RecordDeleteTime
// 	tp.TransCodingRecord.UpdatedAt = tp.RecordUpdateTime
// 	tp.TransCodingRecord.MediaID = tp.RecordMediaID
// 	tp.TransCodingRecord.Reason = tp.RecordReason
// 	tp.TransCodingRecord.Status = tp.RecordStatus
// 	tp.TransCodingRecord.UserName = tp.RecordUserName
// 	tp.TransCodingRecord.TIProjectID = tp.RecordProject
// 	tp.TransCodingRecord.TIBusinessID = tp.RecordTIBusinessID
// 	tp.TransCodingRecord.Priority = tp.RecordPriority
// }

// const taskRecordSelect = "task.create_time as task_create_time, " +
// 	"task.delete_time as task_delete_time, " +
// 	"task.update_time as task_update_time, " +
// 	"task.media_id as task_media_id, " +
// 	"task.reason as task_reason, task.status as task_status, " +
// 	"task.user_name as task_user_name, " +
// 	"task.ti_project_id as task_project, " +
// 	"task.ti_business_id as task_ti_business_id, " +
// 	"task.priority as task_priority, " +
// 	"trans_coding_record.create_time as record_create_time, " +
// 	"trans_coding_record.delete_time as record_delete_time, " +
// 	"trans_coding_record.update_time as record_update_time, " +
// 	"trans_coding_record.media_id as record_media_id, " +
// 	"trans_coding_record.reason as record_reason, " +
// 	"trans_coding_record.status as record_status, " +
// 	"trans_coding_record.user_name as record_user_name, " +
// 	"trans_coding_record.ti_project_id as record_project, " +
// 	"trans_coding_record.ti_business_id as record_ti_business_id, " +
// 	"trans_coding_record.priority as record_priority"

// // GetReadyTasks 按照优先级获取准备好的任务以及转码记录
// func (t TaskDao) GetReadyTasks(ctx context.Context, appIDs []string, limit int, maxSize int, mediaTypes []string) (
// 	taskAndTransCodingRecords []taskAndTransCodingRecord, err error) {
// 	db := config.GetGlobalDB().WithContext(ctx)
// 	// left join,兼容之前直播流没有创建转码记录
// 	err = db.Model(&models.Task{}).Select("task.*, trans_coding_record.*, "+taskRecordSelect).
// 		Joins("left join trans_coding_record on trans_coding_record.media_id = task.media_id "+
// 			"and trans_coding_record.max_size = ?", maxSize).
// 		Where("trans_coding_record.status = ? or trans_coding_record.status = ? or trans_coding_record.status is null",
// 			models.TransCodingStatusSuccessed, models.TransCodingStatusFailed).
// 		Where("task.status = ? and task.delete_time is null and task.app_id in (?) "+
// 			"and media_type in (?)", models.TaskStatusWait, appIDs, mediaTypes).
// 		Limit(limit).Order("task.priority asc, task.flow_start_time asc").Scan(&taskAndTransCodingRecords).Error
// 	for i := range taskAndTransCodingRecords {
// 		taskAndTransCodingRecords[i].reBuildAfterQuery()
// 	}
// 	return taskAndTransCodingRecords, err
// }

// // GetReadyTasks 按照优先级获取准备好的标签分析任务
// func (t TaskDao) GetReadyTagAnalyseTasks(ctx context.Context, limit int) (tasks []models.Task, err error) {
// 	db := config.GetGlobalDB().WithContext(ctx)
// 	// left join,兼容之前直播流没有创建转码记录
// 	err = db.Model(&models.Task{}).Where("status = ? and delete_time is null and app_id = ? ",
// 		models.TaskStatusWait, config.TagAnalyseAppID).
// 		Limit(limit).
// 		Order("priority asc, flow_start_time asc").Find(&tasks).Error
// 	return tasks, err
// }

// // UpdateTaskInfo 更新任务信息
// func (t TaskDao) UpdateTaskInfo(ctx context.Context, info *apicommon.TaskInfo) (err error) {
// 	db := config.GetGlobalDB().WithContext(ctx)
// 	var task models.Task
// 	if e := db.Where("task_id = ?", info.TaskID).First(&task).Error; e != nil {
// 		if errors.Is(e, gorm.ErrRecordNotFound) {
// 			return errors.New("没有该任务")
// 		}
// 		return e
// 	}
// 	task.TaskName = info.TaskName
// 	task.AppID = info.AppID
// 	if task.MediaID != nil {
// 		*task.MediaID = int(info.MediaID)
// 	}
// 	task.Status = info.TasktStatus
// 	current := time.Now()
// 	if info.TasktStatus == models.TaskStatusWait {
// 		task.FlowStartAt = &current
// 		task.FlowEndAt = nil
// 		task.Reason = ""
// 		task.RetryTime = 0
// 	} else if info.TasktStatus == models.TaskStatusDeleted {
// 		task.DeletedAt = &current
// 	} else if info.TasktStatus == models.TaskStatusSuccess {
// 		nowTime := time.Now()
// 		task.FlowEndAt = &nowTime
// 	}
// 	if e := db.Save(&task).Error; e != nil {
// 		return errors.New("更新任务失败")
// 	}
// 	return nil
// }

// // UpdateTaskInfoWithParam 更新任务信息
// func (t TaskDao) UpdateTaskInfoWithParam(ctx context.Context, info *apicommon.TaskInfo,
// 	params map[string]interface{}) (err error) {
// 	db := config.GetGlobalDB().WithContext(ctx)
// 	var task models.Task
// 	if e := db.Where("task_id = ?", info.TaskID).First(&task).Error; e != nil {
// 		if errors.Is(e, gorm.ErrRecordNotFound) {
// 			return errors.New("没有该任务")
// 		}
// 		return e
// 	}
// 	task.TaskName = info.TaskName
// 	task.AppID = info.AppID
// 	if task.MediaID != nil {
// 		*task.MediaID = int(info.MediaID)
// 	}
// 	task.Status = info.TasktStatus
// 	current := time.Now()
// 	if info.TasktStatus == models.TaskStatusWait {
// 		task.FlowStartAt = &current
// 		task.FlowEndAt = nil
// 		task.Reason = ""
// 		task.RetryTime = 0
// 	} else if info.TasktStatus == models.TaskStatusDeleted {
// 		task.DeletedAt = &current
// 	} else if info.TasktStatus == models.TaskStatusSuccess {
// 		nowTime := time.Now()
// 		task.FlowEndAt = &nowTime
// 	}
// 	tx := db.Begin()
// 	ctxLog := applog.AppLog{TaskID: task.TaskID}
// 	if e := tx.Save(&task).Error; e != nil {
// 		ctxLog.Log(log.LevelError, "Update task status failed, ", e)
// 		tx.Rollback()
// 		return errors.New("更新任务失败, err: " + e.Error())
// 	}
// 	if e := tx.Table("task").Where("task_id = ?", task.TaskID).Updates(params).Error; e != nil {
// 		tx.Rollback()
// 		ctxLog.Log(log.LevelError, "Update task status failed, ", e)
// 		return errors.New("更新任务失败, err: " + e.Error())
// 	}
// 	if e := tx.Commit().Error; e != nil {
// 		ctxLog.Log(log.LevelError, "Update task status failed, ", e)
// 		tx.Rollback()
// 		return errors.New("更新任务失败, err: " + e.Error())
// 	}
// 	return nil
// }

// // RemoveTempVideo 删除临时视频
// func (t TaskDao) RemoveTempVideo(ctx context.Context, taskID int) (err error) {
// 	if config.GetConfig().IsDebug {
// 		return nil
// 	}
// 	db := config.GetGlobalDB().WithContext(ctx)
// 	var task models.Task
// 	if e := db.Where("task_id = ?", taskID).First(&task).Error; e != nil {
// 		if errors.Is(e, gorm.ErrRecordNotFound) {
// 			return errors.New("没有该任务")
// 		}
// 		return e
// 	}
// 	return os.Remove(task.TempVideoPath)
// }

// // DeleteByTaskID 获取过期任务
// func (t TaskDao) DeleteByTaskID(ctx context.Context, taskID int) (err error) {
// 	db := config.GetGlobalDB().WithContext(ctx)
// 	return db.Delete(models.Task{}, "task_id = ?", taskID).Error
// }

// // UpdateTaskById 更新任务信息
// func (t TaskDao) UpdateTaskById(ctx context.Context, taskID int,
// 	fieldValues map[string]interface{}) (err error) {
// 	ctxLog := applog.AppLog{TaskID: taskID}

// 	defer func() {
// 		if err != nil {
// 			ctxLog.Log(log.LevelError, err)
// 		}
// 	}()

// 	if taskID <= 0 {
// 		err = fmt.Errorf("UpdateTaskById, taskID[%d] invalid", taskID)
// 		return err
// 	}

// 	db := config.GetGlobalDB().WithContext(ctx)
// 	if e := db.Model(&models.Task{}).Where("task_id = ?", taskID).Updates(fieldValues).Error; e != nil {
// 		err = fmt.Errorf("UpdateTaskById, taskID[%d] invalid", err)
// 		return err
// 	}

// 	return nil
// }

// func (t TaskDao) GetTasksByTaskIds(ctx context.Context, taskIds []int32) ([]models.Task, error) {
// 	var taskList []models.Task
// 	db := config.GetGlobalDB().WithContext(ctx)
// 	err := db.Debug().Where("task_id IN ?", taskIds).Find(&taskList).Error
// 	if err != nil {
// 		return nil, err
// 	}
// 	return taskList, nil
// }
