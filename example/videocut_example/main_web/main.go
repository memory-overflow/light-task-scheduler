package main

import (
	"log"
	"time"

	lighttaskscheduler "github.com/memory-overflow/light-task-scheduler"
	"github.com/memory-overflow/light-task-scheduler/container"
	memeorycontainer "github.com/memory-overflow/light-task-scheduler/container/memory_container"
	videocut "github.com/memory-overflow/light-task-scheduler/example/videocut_example/video_cut"
)

func main() {

	// 构建队列容器，队列长度 10000
	queueContainer := memeorycontainer.MakeQueueContainer(10000, 100*time.Millisecond)
	scanInterval := 50 * time.Millisecond
	// 替换自己的数据库
	sqlContainer, err := videocut.MakeVideoCutSqlContainer("127.0.0.1", "3306", "root", "", "test")
	if err != nil {
		log.Fatalf("build container failed: %v\n", err)
	}
	comb := container.MakeCombinationContainer(queueContainer, sqlContainer)
	// 构建裁剪任务执行器
	actuator := videocut.MakeVideoCutActuator()
	sch, err := lighttaskscheduler.MakeScheduler(
		comb, actuator, sqlContainer,
		lighttaskscheduler.Config{
			TaskLimit:              2,                // 2 并发
			TaskTimeout:            60 * time.Second, // 20s 超时时间
			EnableFinshedTaskList:  true,             // 开启已完成任务返回功能
			SchedulingPollInterval: scanInterval,
			DisableStatePoll:       false,
			StatePollInterval:      scanInterval, // 开启已完成任务返回功能
		},
	)
	if err != nil {
		log.Fatalf("make schedule failed: %v\n", err)
	}
	go videocut.StartServer()                  // start video cut microservice
	videocut.StartWebServer(sqlContainer, sch) // start web server
}
