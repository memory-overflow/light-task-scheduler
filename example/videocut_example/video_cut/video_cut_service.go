package videocut

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// generateRandomString 生成随机字符串
func generateRandomString(length int) string {
	const alphanum = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	rand.Seed(time.Now().UTC().UnixNano())
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		result[i] = alphanum[rand.Intn(len(alphanum))]
	}
	return string(result)
}

type StatusString string

const (
	TASK_STATUS_RUNNING StatusString = "分析中"
	TASK_STATUS_FAILED  StatusString = "分析失败"
	TASK_STATUS_SUCCESS StatusString = "分析成功"
)

type taskStatus struct {
	cancel       context.CancelFunc // use cancel to stop task
	status       StatusString
	failedReason string
}

var runningTaskMap sync.Map // taskId -> taskStatus 的映射
var outMap sync.Map         // taskId -> outputPath 的映射

// VideoCutReq 创建视频裁剪任务请求
type VideoCutReq struct {
	StartTime  float32 // 视频开始时间
	EndTIme    float32 // 视频结束时间
	InputVideo string  // 输入视频地址
}

// VideoCutRsp 创建视频裁剪任务返回
type VideoCutRsp struct {
	TaskId string // 任务 ID
}

func cut(ctx context.Context, taskId, inputVideo string, startTime, endTIme float32) {
	// 输出放同目录，修改文件名
	spt := strings.Split(inputVideo, ".")
	l := len(spt)
	if l > 1 {
		l--
	}
	outputVideo := fmt.Sprintf("%s_cut_%g_%g.mp4", strings.Join(spt[:l], "."), startTime, endTIme)
	// install ffmpeg first
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", inputVideo,
		"-ss", fmt.Sprintf("%f", startTime),
		"-to", fmt.Sprintf("%f", endTIme),
		"-c:v", "libx264",
		"-crf", "30",
		"-y", outputVideo,
	)
	errout := new(bytes.Buffer)
	cmd.Stderr = errout
	if err := cmd.Run(); err != nil {
		os.Remove(outputVideo) // 出错删除输出文件
		err = fmt.Errorf("exec ffmpeg failed: %v, stderr: %v", err, errout)
		log.Printf("%v", err)
		if st, ok := runningTaskMap.Load(taskId); ok {
			st.(taskStatus).cancel() // 执行 cancel 清空 ctx
			runningTaskMap.Store(taskId, taskStatus{
				status:       TASK_STATUS_FAILED,
				failedReason: err.Error(),
			})
		}
	} else {
		if st, ok := runningTaskMap.Load(taskId); ok {
			st.(taskStatus).cancel() // 执行 cancel 清空 ctx
			runningTaskMap.Store(taskId, taskStatus{
				status: TASK_STATUS_SUCCESS,
			})
			outMap.Store(taskId, outputVideo) // 存储输出路径
		} else {
			os.Remove(outputVideo) // 任务找不到删除输出文件
		}
	}
}

func videoCut(w http.ResponseWriter, r *http.Request) {
	input, _ := ioutil.ReadAll(r.Body)
	var req VideoCutReq
	json.Unmarshal(input, &req)
	taskId := "task-" + generateRandomString(8)
	// 添加
	ctx, cancel := context.WithCancel(context.Background())
	status := taskStatus{
		cancel: cancel, // 支持 stop cancel 掉任务
		status: TASK_STATUS_RUNNING,
	}
	runningTaskMap.Store(taskId, status)
	// 异步执行
	go cut(ctx, taskId, req.InputVideo, req.StartTime, req.EndTIme)
	rsp := VideoCutRsp{
		TaskId: taskId,
	}
	bs, _ := json.Marshal(rsp)
	w.Write(bs)
	// log.Printf("success create task, task_id: %s, inputVideo: %s, startTime: %g, endTime: %g\n",
	// 	rsp.TaskId, req.InputVideo, req.StartTime, req.EndTIme)
}

// StatusRequest ...
type StatusRequest struct {
	TaskId string
}

// StatusResponse ...
type StatusResponse struct {
	TaskId string
	Status StatusString
	Reason string
}

func status(w http.ResponseWriter, r *http.Request) {
	input, _ := ioutil.ReadAll(r.Body)
	var req StatusRequest
	json.Unmarshal(input, &req)
	task, ok := runningTaskMap.Load(req.TaskId)
	var rsp StatusResponse
	rsp.TaskId = req.TaskId
	if !ok {
		rsp.Status = TASK_STATUS_FAILED
		rsp.Reason = "任务未找到"
	} else {
		rsp.Status = task.(taskStatus).status
		rsp.Reason = task.(taskStatus).failedReason
	}
	if rsp.Status != TASK_STATUS_RUNNING {
		// 如果任务已经完成，获取状态后删除临时状态
		runningTaskMap.Delete(req.TaskId)
	}

	bs, _ := json.Marshal(rsp)
	w.Write(bs)
}

// GetOutputVideoResponse ...
type GetOutputVideoResponse struct {
	TaskId      string
	OutputVideo string // 输出视频
	Reason      string // 如果找不到输出，返回找不到输出的原因
}

func getOutputVideo(w http.ResponseWriter, r *http.Request) {
	input, _ := ioutil.ReadAll(r.Body)
	var req StatusRequest
	json.Unmarshal(input, &req)
	out, ok := outMap.Load(req.TaskId)
	var rsp GetOutputVideoResponse
	rsp.TaskId = req.TaskId
	if ok {
		rsp.OutputVideo = out.(string)
	} else {
		rsp.Reason = "请求参数错误，未找到任务输出"
	}
	bs, _ := json.Marshal(rsp)
	w.Write(bs)
}

// StopRequest ...
type StopRequest struct {
	TaskId string
}

// StopResponse ...
type StopResponse struct {
	Reason string
}

func stop(w http.ResponseWriter, r *http.Request) {
	input, _ := ioutil.ReadAll(r.Body)
	var req StopRequest
	json.Unmarshal(input, &req)
	status, ok := runningTaskMap.Load(req.TaskId)
	var rsp StopResponse
	if !ok {
		rsp.Reason = "未找到该任务"
	} else {
		if status.(taskStatus).status != TASK_STATUS_RUNNING {
			rsp.Reason = "任务没有在执行中，请查询任务状态"
		} else {
			runningTaskMap.Delete(req.TaskId)
			status.(taskStatus).cancel() // 取消任务
		}
	}
	bs, _ := json.Marshal(rsp)
	w.Write(bs)
}

// StartServer ...
func StartServer() {
	http.HandleFunc("/VideoCut", videoCut)
	http.HandleFunc("/Status", status)
	http.HandleFunc("/GetOutputVideo", getOutputVideo)
	http.HandleFunc("/Stop", stop)
	fmt.Printf("start videocut service ...\n")
	http.ListenAndServe("127.0.0.1:8000", nil)
}
