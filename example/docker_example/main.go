package main

import (
	"log"
	"strconv"
	"time"

	"context"

	lighttaskscheduler "github.com/memory-overflow/light-task-scheduler"
	"github.com/memory-overflow/light-task-scheduler/actuator"
	memeorycontainer "github.com/memory-overflow/light-task-scheduler/container/memory_container"
)

func main() {
	container := memeorycontainer.MakeQueueContainer(10000, 100*time.Millisecond)
	scanInterval := 50 * time.Millisecond
	// 构建裁剪任务执行器
	act := actuator.MakeDockerActuator(nil)
	sch, _ := lighttaskscheduler.MakeScheduler(
		container, act, nil,
		lighttaskscheduler.Config{
			TaskLimit:              2,                // 2 并发
			TaskTimeout:            60 * time.Second, // 60s 超时时间
			SchedulingPollInterval: scanInterval,
			StatePollInterval:      scanInterval,
			EnableFinshedTaskList:  true,
		},
	)

	for i := 0; i < 10; i++ {
		sch.AddTask(context.Background(),
			lighttaskscheduler.Task{
				TaskId: strconv.Itoa(i),
				TaskItem: actuator.DockerTask{
					Image:         "centos:7",
					Cmd:           []string{"sh", "-c", "echo 'helloworld'; sleep 1"},
					ContainerName: "helloworld-" + strconv.Itoa(i),
					VolumeBinds: map[string]string{
						"/home": "/data",
					},
					MemoryLimit: 10 * 1024 * 1024, // 10 MB
					CpuPercent:  100,
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
