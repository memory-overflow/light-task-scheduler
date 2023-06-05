package main

import (
	"context"
	"log"
	"time"

	lighttaskscheduler "github.com/memory-overflow/light-task-scheduler"
	"github.com/memory-overflow/light-task-scheduler/container"
	memeorycontainer "github.com/memory-overflow/light-task-scheduler/container/memory_container"
	videocut "github.com/memory-overflow/light-task-scheduler/example/videocut_example/video_cut"
)

func main() {

	// 构建队列容器，队列长度 10000
	queueContainer := memeorycontainer.MakeQueueContainer(10000, 100*time.Millisecond, nil)
	scanInterval := 50 * time.Millisecond
	// 替换自己的数据库
	sqlContainer, err := videocut.MakeVideoCutSqlContainer("127.0.0.1", "3306", "root", "", "test")
	if err != nil {
		log.Fatalf("build container failed: %v\n", err)
	}

	comb := container.MakeCombinationContainer(queueContainer, sqlContainer)
	// 构建裁剪任务执行器
	actuator := videocut.MakeVideoCutActuator()
	sch := lighttaskscheduler.MakeNewScheduler(
		context.Background(),
		comb, actuator,
		lighttaskscheduler.Config{
			TaskLimit:             2, // 2 并发
			ScanInterval:          scanInterval,
			TaskTimeout:           120 * time.Second, // 120s 超时时间
			EnableFinshedTaskList: false,             // 开启已完成任务返回功能
		},
	)

	go videocut.StartServer()                  // start video cut microservice
	videocut.StartWebServer(sqlContainer, sch) // start web server
}
