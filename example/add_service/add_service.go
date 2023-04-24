package service

import (
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

var id int32 = 0

var taskMap sync.Map
var taskCh chan AddTask = make(chan AddTask, 100)

// AddTask ..
type AddTask struct {
	A, B   uint32
	Res    uint32
	TaskId int32
	status string
}

// AddRequest ...
type AddRequest struct {
	A, B uint32
}

// AddResponse ...
type AddResponse struct {
	TaskId int32
}

func worker() {
	for task := range taskCh {
		task.status = "running"
		taskMap.Store(task.TaskId, task)
		time.Sleep(time.Duration(rand.Intn(5000)) * time.Millisecond)
		task.Res = task.A + task.B
		task.status = "success"
		taskMap.Store(task.TaskId, task)
	}
}

func add(w http.ResponseWriter, r *http.Request) {
	input, _ := ioutil.ReadAll(r.Body)
	var req AddRequest
	json.Unmarshal(input, &req)
	atomic.AddInt32(&id, 1)
	t := AddTask{
		A:      req.A,
		B:      req.B,
		TaskId: id,
		status: "unstart",
	}
	taskCh <- t
	taskMap.Store(t.TaskId, t)
	rsp := AddResponse{
		TaskId: t.TaskId,
	}
	bs, _ := json.Marshal(rsp)
	w.Write(bs)
}

// StatusRequest ...
type StatusRequest struct {
	TaskId int32
}

// StatusResponse ...
type StatusResponse struct {
	Status string
}

func status(w http.ResponseWriter, r *http.Request) {
	input, _ := ioutil.ReadAll(r.Body)
	var req StatusRequest
	json.Unmarshal(input, &req)
	task, ok := taskMap.Load(req.TaskId)
	var rsp StatusResponse
	if !ok {
		rsp.Status = "not found"
	} else {
		rsp.Status = task.(AddTask).status
	}
	bs, _ := json.Marshal(rsp)
	w.Write(bs)
}

// ResultRequest ...
type ResultRequest struct {
	TaskId int32
}

// ResultResponse ...
type ResultResponse struct {
	Res uint32
}

func result(w http.ResponseWriter, r *http.Request) {
	input, _ := ioutil.ReadAll(r.Body)
	var req ResultRequest
	json.Unmarshal(input, &req)
	task, ok := taskMap.Load(req.TaskId)
	var rsp ResultResponse
	if !ok {
		rsp.Res = 0
	} else {
		rsp.Res = task.(AddTask).Res
	}
	bs, _ := json.Marshal(rsp)
	w.Write(bs)
}

// StartServer ...
func StartServer() {
	http.HandleFunc("/add", add)
	http.HandleFunc("/status", status)
	http.HandleFunc("/result", result)
	for i := 0; i < 2; i++ {
		go worker()
	}
	http.ListenAndServe("127.0.0.1:8000", nil)
}
