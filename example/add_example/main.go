package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"time"

	lighttaskscheduler "github.com/memory-overflow/light-task-scheduler"
	"github.com/memory-overflow/light-task-scheduler/actuator"
	memeorycontainer "github.com/memory-overflow/light-task-scheduler/container/memory_container"
)

// AddTask add 任务结构
type AddTask struct {
	StartTime time.Time
	A, B      int32
}

func add(ctx context.Context, ftask *lighttaskscheduler.Task) (data interface{}, err error) {
	task, ok := ftask.TaskItem.(AddTask)
	if !ok {
		return nil, fmt.Errorf("TaskItem not be set to AddTask")
	}
	log.Printf("start run task %s, Attempts: %d\n", ftask.TaskId, ftask.TaskAttemptsTime)
	// 模拟 25 % 的概率出错
	if rand.Intn(4) == 0 {
		return nil, fmt.Errorf("error test")
	}
	// 模拟 25% 概率超时
	time.Sleep(time.Duration(rand.Intn(4000))*time.Millisecond + 2*time.Second)

	return task.A + task.B, nil
}

type callbackReceiver struct {
}

// modePolling 默认模式，状态轮询模式
func modePolling() *lighttaskscheduler.TaskScheduler {
	container := memeorycontainer.MakeQueueContainer(10000, 100*time.Millisecond)
	actuator, err := actuator.MakeFucntionActuator(add, nil)
	if err != nil {
		log.Fatal("make fucntionActuato error: ", err)
	}
	sch, err := lighttaskscheduler.MakeScheduler(
		container, actuator, nil,
		lighttaskscheduler.Config{
			TaskLimit:              2,
			TaskTimeout:            5 * time.Second, // 5s 超时
			EnableFinshedTaskList:  true,            // 开启已完成任务返回功能
			MaxFailedAttempts:      3,               // 失败重试次数
			SchedulingPollInterval: 0,
			StatePollInterval:      0,
		},
	)
	if err != nil {
		log.Fatal("make scheduler error: ", err)
	}
	return sch
}

type addCallbackReceiver struct {
	taskChannel chan lighttaskscheduler.Task
}

func (rec addCallbackReceiver) GetCallbackChannel(ctx context.Context) chan lighttaskscheduler.Task {
	return rec.taskChannel
}

// modeCallback 仅回调模式
func modeCallback() *lighttaskscheduler.TaskScheduler {
	container := memeorycontainer.MakeQueueContainer(10000, 100*time.Millisecond)
	taskChannel := make(chan lighttaskscheduler.Task, 10000)
	actuator, err := actuator.MakeFucntionActuator(add, nil)
	actuator.SetCallbackChannel(taskChannel)
	if err != nil {
		log.Fatal("make fucntionActuato error: ", err)
	}
	receiver := addCallbackReceiver{
		taskChannel: taskChannel,
	}
	sch, err := lighttaskscheduler.MakeScheduler(
		container, actuator, nil,
		lighttaskscheduler.Config{
			TaskLimit:              2,
			TaskTimeout:            5 * time.Second, // 5s 超时
			EnableFinshedTaskList:  true,            // 开启已完成任务返回功能
			MaxFailedAttempts:      3,               // 失败重试次数
			SchedulingPollInterval: 0,
			DisableStatePoll:       true,
			EnableStateCallback:    true,
			CallbackReceiver:       receiver,
		},
	)
	if err != nil {
		log.Fatal("make scheduler error: ", err)
	}
	return sch
}

// modeCallback 轮询 + 回调模式
func modePollingCallback() *lighttaskscheduler.TaskScheduler {
	container := memeorycontainer.MakeQueueContainer(10000, 100*time.Millisecond)
	taskChannel := make(chan lighttaskscheduler.Task, 10000)
	actuator, err := actuator.MakeFucntionActuator(add, nil)
	actuator.SetCallbackChannel(taskChannel)
	if err != nil {
		log.Fatal("make fucntionActuato error: ", err)
	}
	receiver := addCallbackReceiver{
		taskChannel: taskChannel,
	}
	sch, err := lighttaskscheduler.MakeScheduler(
		container, actuator, nil,
		lighttaskscheduler.Config{
			TaskLimit:              2,
			TaskTimeout:            5 * time.Second, // 5s 超时
			EnableFinshedTaskList:  true,            // 开启已完成任务返回功能
			MaxFailedAttempts:      3,               // 失败重试次数
			SchedulingPollInterval: 0,
			DisableStatePoll:       false,
			StatePollInterval:      0,
			EnableStateCallback:    true,
			CallbackReceiver:       receiver,
		},
	)
	if err != nil {
		log.Fatal("make scheduler error: ", err)
	}
	return sch
}

func main() {
	// 模式1, 默认模式，状态轮询模式
	// sch := modePolling()

	// 模式2, 回调模式
	// sch := modeCallback()

	// 模式3，轮询 + 回调模式
	sch := modePollingCallback()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 10; i++ {
		sch.AddTask(context.Background(),
			lighttaskscheduler.Task{
				TaskId: strconv.Itoa(i),
				TaskItem: AddTask{
					A: r.Int31() % 1000,
					B: r.Int31() % 1000,
				},
			})
	}
	for task := range sch.FinshedTasks() {
		if task.TaskStatus == lighttaskscheduler.TASK_STATUS_FAILED {
			log.Printf("failed task %s, reason: %s, timecost: %dms, attempt times: %d\n",
				task.TaskId, task.FailedReason, task.TaskEnbTime.Sub(task.TaskStartTime).Milliseconds(), task.TaskAttemptsTime)
		} else if task.TaskStatus == lighttaskscheduler.TASK_STATUS_SUCCESS {
			log.Printf("success task %s, timecost: %dms, attempt times: %d\n", task.TaskId,
				task.TaskEnbTime.Sub(task.TaskStartTime).Milliseconds(), task.TaskAttemptsTime)
		}
	}

}
