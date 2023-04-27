package videocut

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/memory-overflow/go-common-library/httpcall"
	framework "github.com/memory-overflow/light-task-scheduler"
)

// VideoCutActuator 视频裁剪执行器
type VideoCutActuator struct {
	EndPoint string
}

// MakeVideoCutActuator 构造执行器
func MakeVideoCutActuator() *VideoCutActuator {
	return &VideoCutActuator{
		EndPoint: "http://127.0.0.1:8000",
	}
}

// Init 任务在被调度前的初始化工作
func (v *VideoCutActuator) Init(ctx context.Context, ftask *framework.Task) (
	newTask *framework.Task, err error) {
	// 初始化检查任务参数
	task, ok := ftask.TaskItem.(VideoCutTask)
	if !ok {
		return ftask, fmt.Errorf("TaskItem not be set to AddTask")
	}
	if _, err := os.Stat(task.InputVideo); err != nil {
		return ftask, fmt.Errorf("InputVideo does not exist")
	}
	if task.CutStartTime > task.CutEndTime {
		return ftask, fmt.Errorf("error: CutStartTime after CutEndTime")
	}
	return ftask, nil
}

// Start 执行任务
func (e *VideoCutActuator) Start(ctx context.Context, ftask *framework.Task) (
	newTask *framework.Task, ignoreErr bool, err error) {
	task, ok := ftask.TaskItem.(VideoCutTask)
	if !ok {
		return ftask, false, fmt.Errorf("TaskItem not be set to AddTask")
	}
	req := VideoCutReq{
		InputVideo: task.InputVideo,
		StartTime:  task.CutStartTime,
		EndTIme:    task.CutEndTime,
	}
	var rsp VideoCutRsp
	err = httpcall.JsonPost(ctx, e.EndPoint+"/VideoCut", nil, req, &rsp)
	if err != nil {
		return nil, true, err // 网络问题导致调度，忽略错误，重试调度
	}

	task.TaskId = rsp.TaskId
	ftask.TaskItem = task
	log.Printf("[fTaskId:%s, cutTaskId:%s]start success\n", ftask.TaskId, rsp.TaskId)
	return ftask, false, nil
}

// ExportOutput 导出任务输出，自行处理任务结果
func (e *VideoCutActuator) ExportOutput(ctx context.Context, ftask *framework.Task) error {
	task, ok := ftask.TaskItem.(VideoCutTask)
	if !ok {
		return fmt.Errorf("TaskItem not be set to VideoCutTask")
	}
	req := StatusRequest{
		TaskId: task.TaskId,
	}
	var rsp GetOutputVideoResponse
	err := httpcall.JsonPost(ctx, e.EndPoint+"/GetOutputVideo", nil, req, &rsp)
	if err != nil {
		log.Printf("[fTaskId:%s, cutTaskId:%s]get video output error: %v\n",
			ftask.TaskId, task.TaskId, err)
		return err
	}
	if rsp.Reason != "" {
		err = fmt.Errorf("[TaskId:%s]get video output error: %s", task.TaskId, rsp.Reason)
		return err
	}
	log.Printf("[fTaskId:%s, cutTaskId:%s]finished cut %s from %g to %g, output: %s, timecost: %ds",
		ftask.TaskId, task.TaskId, task.InputVideo, task.CutStartTime, task.CutEndTime,
		rsp.OutputVideo, (time.Now().Unix() - ftask.TaskStartTime.Unix()))
	return nil
}

// Stop 停止任务
func (e *VideoCutActuator) Stop(ctx context.Context, ftask *framework.Task) error {
	task, ok := ftask.TaskItem.(VideoCutTask)
	if !ok {
		return fmt.Errorf("TaskItem not be set to VideoCutTask")
	}
	req := StopRequest{
		TaskId: task.TaskId,
	}
	var rsp StopResponse
	err := httpcall.JsonPost(ctx, e.EndPoint+"/Stop", nil, req, &rsp)
	if err != nil {
		log.Printf("[fTaskId:%s, cutTaskId:%s]stop task error: %v\n",
			ftask.TaskId, task.TaskId, err)
		return err
	}
	if rsp.Reason != "" {
		err = fmt.Errorf("[fTaskId:%s, cutTaskId:%s]stop task error: %s",
			ftask.TaskId, task.TaskId, rsp.Reason)
		return err
	}
	log.Printf("[fTaskId:%s, cutTaskId:%s]stop task success", ftask.TaskId, task.TaskId)
	return nil
}

// GetAsyncTaskStatus 获取任务状态
func (e *VideoCutActuator) GetAsyncTaskStatus(ctx context.Context, tasks []framework.Task) (
	status []framework.AsyncTaskStatus, err error) {
	status = make([]framework.AsyncTaskStatus, len(tasks))
	wg := sync.WaitGroup{}
	for i := range tasks {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			task, ok := tasks[index].TaskItem.(VideoCutTask)
			if !ok {
				status[index] = framework.AsyncTaskStatus{
					TaskStatus:   framework.TASK_STATUS_FAILED,
					FailedReason: errors.New("system error: TaskItem not be set to VideoCutTask"),
					Progress:     float32(0.0),
				}
				return
			}

			req := StatusRequest{
				TaskId: task.TaskId,
			}
			var rsp StatusResponse
			err := httpcall.JsonPost(ctx, e.EndPoint+"/Status", nil, req, &rsp)
			if err != nil {
				log.Printf("[fTaskId:%s, cutTaskId:%s]get task status: %v\n", tasks[index].TaskId, task.TaskId, err)
				status[index] = framework.AsyncTaskStatus{
					TaskStatus: framework.TASK_STATUS_RUNNING,
					Progress:   float32(0.0),
				} // 网络错误，当成继续执行
				return
			}
			if rsp.Reason != "" {
				status[index] = framework.AsyncTaskStatus{
					TaskStatus:   framework.TASK_STATUS_FAILED,
					FailedReason: fmt.Errorf("get task status failed: " + rsp.Reason),
					Progress:     float32(0.0),
				}
				return
			}
			if rsp.Status == TASK_STATUS_RUNNING {
				status[index] = framework.AsyncTaskStatus{
					TaskStatus: framework.TASK_STATUS_RUNNING,
					Progress:   float32(0.0),
				}
			} else if rsp.Status == TASK_STATUS_SUCCESS {
				status[index] = framework.AsyncTaskStatus{
					TaskStatus: framework.TASK_STATUS_SUCCESS,
					Progress:   float32(100.0),
				}
			} else if rsp.Status == TASK_STATUS_FAILED {
				status[index] = framework.AsyncTaskStatus{
					TaskStatus: framework.TASK_STATUS_FAILED,
					Progress:   float32(0.0),
				}
			}
		}(i)
	}
	wg.Wait()
	return status, nil
}

// GetOutput 提供业务查询任务结果的接口
func (e *VideoCutActuator) GetOutput(ctx context.Context, ftask *framework.Task) (
	data interface{}, err error) {
	return nil, nil
}

// GetOutput ...
func (e *VideoCutActuator) Delete(ctx context.Context, ftask *framework.Task) (err error) {
	return e.Stop(ctx, ftask)
}
