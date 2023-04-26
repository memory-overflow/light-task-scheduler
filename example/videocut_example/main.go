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
	go videocut.StartServer() // start video cut microservice

	container := memeorycontainer.MakeQueueContainer(10000, 100*time.Millisecond)
	actuator := videocut.MakeVideoCutActuator()
	sch := lighttaskscheduler.MakeNewScheduler(
		context.Background(),
		container, actuator,
		lighttaskscheduler.Config{
			TaskLimit:    2,
			ScanInterval: 50 * time.Millisecond,
			TaskTimeout:  20 * time.Second, // 20s 超时
		},
	)

	var c chan os.Signal
	for i := 100; i < 200; i += 10 {
		select {
		case <-c:
			return
		default:
			sch.AddTask(context.Background(),
				lighttaskscheduler.Task{
					TaskId: strconv.Itoa(i),
					TaskItem: videocut.VideoCutTask{
						InputVideo:   "/data/workspace/ai-media/media-ai-ppl/video/benpaobaxiongdi_S2EP12_20150703.mp4",
						CutStartTime: 10,
						CutEndTime:   float32(i),
					},
				})
		}
	}

	for range c {
		log.Println("stop Scheduling")
		sch.Close()
		return
	}
}
