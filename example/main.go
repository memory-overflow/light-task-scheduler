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
	"github.com/memory-overflow/light-task-scheduler/example/actuator"
	service "github.com/memory-overflow/light-task-scheduler/example/add_service"
	"github.com/memory-overflow/light-task-scheduler/example/task"
)

func main() {
	go service.StartServer()
	container := memeorycontainer.MakeQueueContainer(10000, 100*time.Millisecond)
	actuator := actuator.MakeExampleActuator()
	sch := lighttaskscheduler.MakeNewScheduler(
		context.Background(),
		container, actuator,
		lighttaskscheduler.Config{
			TaskLimit:    5,
			ScanInterval: 50 * time.Millisecond})

	var c chan os.Signal
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 1000; i++ {
		select {
		case <-c:
			return
		default:
			sch.AddTask(context.Background(),
				lighttaskscheduler.Task{
					TaskId: strconv.Itoa(i),
					TaskItem: task.ExampleTask{
						TaskId: uint32(i),
						A:      r.Int31() % 1000,
						B:      r.Int31() % 1000,
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
