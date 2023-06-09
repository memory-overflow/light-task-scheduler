package videocut

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	framework "github.com/memory-overflow/light-task-scheduler"
)

type webService struct {
	sqldao *videoCutSqlContainer
	sch    *framework.TaskScheduler
}

type TaskInfo struct {
	TaskId       string
	InputVideo   string
	OutputVideo  string
	CutStartTime float32
	CutEndTime   float32
	Status       string
	FailedReason string
	StartTime    string
	EndTime      string
}

type CreateTaskReq struct {
	InputVideo   string
	CutStartTime float32
	CutEndTime   float32
}

type CreateTaskRsp struct {
	TaskId       string
	ErrorCode    int
	ErrorMessage string
}

type TaskListReq struct {
	PageSize   int
	PageNumber int
}

type TaskListRsp struct {
	Tasks        []TaskInfo
	TotalCount   int
	ErrorCode    int
	ErrorMessage string
}

type TaskOpReq struct {
	TaskId string
}

type TaskOpRsp struct {
	TaskId       string
	ErrorCode    int
	ErrorMessage string
}

func getTaskInfoByTask(task *VideoCutTask) (info TaskInfo) {
	m := map[framework.TaskStatus]string{
		framework.TASK_STATUS_INVALID: "未知状态",
		framework.TASK_STATUS_UNSTART: "未开始",
		framework.TASK_STATUS_WAITING: "等待执行",
		framework.TASK_STATUS_RUNNING: "执行中",
		framework.TASK_STATUS_SUCCESS: "执行成功",
		framework.TASK_STATUS_FAILED:  "执行失败",
		framework.TASK_STATUS_STOPED:  "已暂停",
	}
	info.TaskId = task.TaskId
	info.InputVideo = task.InputVideo
	info.OutputVideo = task.OutputVideo
	info.CutStartTime = task.CutStartTime
	info.CutEndTime = task.CutEndTime
	info.Status = m[task.Status]
	info.FailedReason = task.FailedReason
	if task.StartAt != nil {
		info.StartTime = task.StartAt.Format("2006-01-02 15:04:05")
	}
	if task.EndAt != nil {
		info.EndTime = task.EndAt.Format("2006-01-02 15:04:05")
	}

	return info
}

func (web webService) taskList(w http.ResponseWriter, r *http.Request) {
	input, _ := ioutil.ReadAll(r.Body)
	var req TaskListReq
	json.Unmarshal(input, &req)
	rsp := TaskListRsp{
		ErrorCode:    0,
		ErrorMessage: "success",
	}
	tasks, count, err := web.sqldao.GetTasks(context.Background(), req.PageNumber, req.PageSize)
	if err != nil {
		rsp.ErrorCode = 1001
		rsp.ErrorMessage = err.Error()
	}
	rsp.TotalCount = count
	for _, task := range tasks {
		rsp.Tasks = append(rsp.Tasks, getTaskInfoByTask(&task))
	}
	bs, _ := json.Marshal(rsp)
	w.Write(bs)
}

func (web webService) createTask(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	input, _ := ioutil.ReadAll(r.Body)
	var req CreateTaskReq
	json.Unmarshal(input, &req)
	rsp := CreateTaskRsp{
		ErrorCode:    0,
		ErrorMessage: "success",
	}
	defer func() {
		bs, _ := json.Marshal(rsp)
		w.Write(bs)
	}()
	if req.CutStartTime < 0 || req.CutStartTime >= req.CutEndTime {
		rsp.ErrorCode = 1004
		rsp.ErrorMessage = "裁剪时间不符合规范"
		return
	}
	taskId := "task-" + GenerateRandomString(8)
	err := web.sqldao.CreateTask(ctx, VideoCutTask{
		TaskId:       taskId,
		InputVideo:   req.InputVideo,
		CutStartTime: req.CutStartTime,
		CutEndTime:   req.CutEndTime,
		Status:       framework.TASK_STATUS_UNSTART,
	})
	if err != nil {
		rsp.ErrorCode = 1001
		rsp.ErrorMessage = err.Error()
	} else {
		rsp.TaskId = taskId
	}
}

func (web webService) startTask(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	input, _ := ioutil.ReadAll(r.Body)
	var req TaskOpReq
	json.Unmarshal(input, &req)
	rsp := TaskOpRsp{
		ErrorCode:    0,
		ErrorMessage: "success",
	}
	defer func() {
		bs, _ := json.Marshal(rsp)
		w.Write(bs)
	}()
	task, err := web.sqldao.GetTask(ctx, req.TaskId)
	if err != nil {
		rsp.ErrorCode = 1001
		rsp.ErrorMessage = err.Error()
		return
	}
	if task.Status == framework.TASK_STATUS_EXPORTING ||
		task.Status == framework.TASK_STATUS_WAITING ||
		task.Status == framework.TASK_STATUS_RUNNING {
		rsp.ErrorCode = 1002
		rsp.ErrorMessage = "任务已经在执行中或者等待队列中"
		return
	}
	err = web.sch.AddTask(ctx, framework.Task{
		TaskId:   task.TaskId,
		TaskItem: *task,
	})
	if err != nil {
		rsp.ErrorCode = 1001
		rsp.ErrorMessage = err.Error()
		return
	}
	rsp.TaskId = task.TaskId
}

func (web webService) stopTask(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	input, _ := ioutil.ReadAll(r.Body)
	var req TaskOpReq
	json.Unmarshal(input, &req)
	rsp := TaskOpRsp{
		ErrorCode:    0,
		ErrorMessage: "success",
	}
	defer func() {
		bs, _ := json.Marshal(rsp)
		w.Write(bs)
	}()
	task, err := web.sqldao.GetTask(ctx, req.TaskId)
	if err != nil {
		rsp.ErrorCode = 1001
		rsp.ErrorMessage = err.Error()
		return
	}
	if task.Status == framework.TASK_STATUS_EXPORTING ||
		task.Status == framework.TASK_STATUS_WAITING ||
		task.Status == framework.TASK_STATUS_RUNNING {
		err = web.sch.StopTask(ctx, &framework.Task{
			TaskId:     task.TaskId,
			TaskItem:   *task,
			TaskStatus: task.Status,
		})
		if err != nil {
			rsp.ErrorCode = 1001
			rsp.ErrorMessage = err.Error()
			return
		}
	} else {
		rsp.ErrorCode = 1002
		rsp.ErrorMessage = "任务不在分析中"
		return
	}
	rsp.TaskId = task.TaskId
}

func (web webService) deleteTask(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	input, _ := ioutil.ReadAll(r.Body)
	var req TaskOpReq
	json.Unmarshal(input, &req)
	rsp := TaskOpRsp{
		ErrorCode:    0,
		ErrorMessage: "success",
	}
	defer func() {
		bs, _ := json.Marshal(rsp)
		w.Write(bs)
	}()
	task, err := web.sqldao.GetTask(ctx, req.TaskId)
	if err != nil {
		rsp.ErrorCode = 1001
		rsp.ErrorMessage = err.Error()
		return
	}
	if task.Status == framework.TASK_STATUS_EXPORTING ||
		task.Status == framework.TASK_STATUS_WAITING ||
		task.Status == framework.TASK_STATUS_RUNNING {
		rsp.ErrorCode = 1003
		rsp.ErrorMessage = "执行中、等待中的任务无法直接删除"
		return
	}
	_, err = web.sqldao.ToDeleteStatus(ctx, &framework.Task{
		TaskId:   task.TaskId,
		TaskItem: *task,
	})
	if err != nil {
		rsp.ErrorCode = 1001
		rsp.ErrorMessage = err.Error()
		return
	}
	rsp.TaskId = task.TaskId
}

func html(w http.ResponseWriter, r *http.Request) {
	// 获取请求的文件路径
	path := r.URL.Path[1:]
	if strings.HasSuffix(path, "js") {
		w.Write([]byte(jsFile))
	} else {
		w.Write([]byte(htmlFile))
	}
}

func download(w http.ResponseWriter, r *http.Request) {
	// 获取请求的文件路径
	path := r.URL.Query().Get("path")
	file, err := os.Open(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 如果请求的是图片或视频文件，则设置 Content-Type 为相应的 MIME 类型
	if strings.HasSuffix(path, ".jpg") || strings.HasSuffix(path, ".jpeg") {
		w.Header().Set("Content-Type", "image/jpeg")
	} else if strings.HasSuffix(path, ".png") {
		w.Header().Set("Content-Type", "image/png")
	} else if strings.HasSuffix(path, ".gif") {
		w.Header().Set("Content-Type", "image/gif")
	} else if strings.HasSuffix(path, ".mp4") {
		w.Header().Set("Content-Type", "video/mp4")
	}

	http.ServeContent(w, r, fileInfo.Name(), fileInfo.ModTime(), file)
	return
}

func StartWebServer(container *videoCutSqlContainer, sch *framework.TaskScheduler) {
	ws := webService{
		sqldao: container,
		sch:    sch,
	}
	http.HandleFunc("/html/", html)
	http.HandleFunc("/Download", download)

	http.HandleFunc("/TaskList", ws.taskList)
	http.HandleFunc("/CreateTask", ws.createTask)
	http.HandleFunc("/StartTask", ws.startTask)
	http.HandleFunc("/StopTask", ws.stopTask)
	http.HandleFunc("/DeleteTask", ws.deleteTask)

	fmt.Printf("start web service ...\n")
	http.ListenAndServe("127.0.0.1:8000", nil)
}
