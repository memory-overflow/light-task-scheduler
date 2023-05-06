package main

import (
	"context"
	"log"
	"math/rand"
	"strconv"
	"time"

	lighttaskscheduler "github.com/memory-overflow/light-task-scheduler"
	memeorycontainer "github.com/memory-overflow/light-task-scheduler/container/memory_container"

	addexample "github.com/memory-overflow/light-task-scheduler/example/add_example/add"
)

func main() {
	save := func(ctx context.Context, ftask *lighttaskscheduler.Task, data interface{}) error {
		log.Printf("save task %s: result = %d \n", ftask.TaskId, data.(int32))
		return nil
	}
	container := memeorycontainer.MakeQueueContainer(10000, 100*time.Millisecond, save)
	actuator := &addexample.AddActuator{}
	sch := lighttaskscheduler.MakeNewScheduler(
		context.Background(),
		container, actuator,
		lighttaskscheduler.Config{
			TaskLimit:             5,
			ScanInterval:          50 * time.Millisecond,
			TaskTimeout:           5 * time.Second, // 5s 超时
			EnableFinshedTaskList: true,            // 开启已完成任务返回功能
		},
	)

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 10; i++ {
		sch.AddTask(context.Background(),
			lighttaskscheduler.Task{
				TaskId: strconv.Itoa(i),
				TaskItem: addexample.AddTask{
					A: r.Int31() % 1000,
					B: r.Int31() % 1000,
				},
			})
	}
	for task := range sch.FinshedTasks() {
		if task.TaskStatus == lighttaskscheduler.TASK_STATUS_FAILED {
			log.Printf("failed task %s, reason: %s, timecost: %dms\n",
				task.TaskId, task.FailedReason, task.TaskEnbTime.Sub(task.TaskStartTime).Milliseconds())
		} else if task.TaskStatus == lighttaskscheduler.TASK_STATUS_SUCCESS {
			log.Printf("success task %s, timecost: %dms\n", task.TaskId,
				task.TaskEnbTime.Sub(task.TaskStartTime).Milliseconds())
		}
	}
}
