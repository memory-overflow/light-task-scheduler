package main

import (
	"context"
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"

	lighttaskscheduler "github.com/memory-overflow/light-task-scheduler"
	memeorycontainer "github.com/memory-overflow/light-task-scheduler/container/memory_container"

	addexample "github.com/memory-overflow/light-task-scheduler/example/add_example/add"
)

func main() {
	container := memeorycontainer.MakeQueueContainer(10000, 100*time.Millisecond)
	actuator := &addexample.AddActuator{}
	sch := lighttaskscheduler.MakeNewScheduler(
		context.Background(),
		container, actuator,
		lighttaskscheduler.Config{
			TaskLimit:    5,
			ScanInterval: 50 * time.Millisecond,
			TaskTimeout:  5 * time.Second, // 5s 超时
		},
	)

	var c chan os.Signal
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 10; i++ {
		select {
		case <-c:
			return
		default:
			sch.AddTask(context.Background(),
				lighttaskscheduler.Task{
					TaskId: strconv.Itoa(i),
					TaskItem: addexample.AddTask{
						A: r.Int31() % 1000,
						B: r.Int31() % 1000,
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
