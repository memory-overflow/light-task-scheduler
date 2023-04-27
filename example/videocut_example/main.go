package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	lighttaskscheduler "github.com/memory-overflow/light-task-scheduler"
	memeorycontainer "github.com/memory-overflow/light-task-scheduler/container/memory_container"
	videocut "github.com/memory-overflow/light-task-scheduler/example/videocut_example/video_cut"
)

func main() {
	inputVideo := os.Args[1]
	go videocut.StartServer() // start video cut microservice

	// 构建队列容器，队列长度 10000
	container := memeorycontainer.MakeQueueContainer(10000, 100*time.Millisecond)
	// 构建裁剪任务执行器
	actuator := videocut.MakeVideoCutActuator()
	sch := lighttaskscheduler.MakeNewScheduler(
		context.Background(),
		container, actuator,
		lighttaskscheduler.Config{
			TaskLimit:    2, // 2 并发
			ScanInterval: 50 * time.Millisecond,
			TaskTimeout:  20 * time.Second, // 20s 超时时间
		},
	)

	// 添加任务，把视频裁前 100s 剪成 10s 一个的视频
	var c chan os.Signal
	for i := 0; i < 100; i += 10 {
		select {
		case <-c:
			return
		default:
			if err := sch.AddTask(context.Background(),
				lighttaskscheduler.Task{
					TaskId: strconv.Itoa(i), // 这里的任务 ID 是为了调度框架方便标识唯一任务的ID, 和微服务的任务ID不同，是映射关系
					TaskItem: videocut.VideoCutTask{
						InputVideo:   inputVideo,
						CutStartTime: float32(i),
						CutEndTime:   float32(i + 10),
					},
				}); err != nil {
				log.Printf("add task TaskId %s failed: %v\n", strconv.Itoa(i), err)
			}
		}
	}

	for range c {
		log.Println("stop Scheduling")
		sch.Close()
		return
	}
}
