package lighttaskscheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Config 配置
type Config struct {
	TaskLimit int32 // 任务并发限制
	// ScanInterval 任务调度期会使用任务容器的接口定期获取任务等待队列和执行中的任务，进行调度和更新操作。
	// 如果是 db 类的 TaskContainer, 可能涉及到扫 db，可以适当配置大一点。
	ScanInterval time.Duration
	TaskTimeout  time.Duration // 任务超时时间

	// 是否需要调度器返回已完成的任务
	// 如果为 true，需要通过
	// for finishedTask := range TaskScheduler.FinshedTasks() {
	//   ...
	// }
	// 及时取走 channel 中的数据，否则可能造成 channel 满了，任务调度阻塞
	// 默认不开启，为 false
	EnableFinshedTaskList bool
}

// TaskScheduler 任务调度器，通过对任务容器和任务执行器的操作，实现任务调度
type TaskScheduler struct {
	// Container 配置的任务容器
	Container TaskContainer
	// Actuator 配置的任务执行器
	Actuator TaskActuator

	config      Config
	ctx         context.Context
	finshedTask chan *Task
}

// MakeNewScheduler 新建任务调度器
func MakeNewScheduler(
	ctx context.Context,
	container TaskContainer,
	actuator TaskActuator,
	config Config) *TaskScheduler {
	scheduler := &TaskScheduler{
		Container: container,
		Actuator:  actuator,
		config:    config,
		ctx:       ctx,
	}
	if config.EnableFinshedTaskList {
		scheduler.finshedTask = make(chan *Task, 10000)
	}
	go scheduler.start()
	return scheduler
}

// AddTask 添加一个任务，需要把任务转换成 lighttaskscheduler.Task 的通用形式
// 注意一定要配置一个唯一的任务 id 标识
func (s *TaskScheduler) AddTask(ctx context.Context, task Task) error {
	newTask, err := s.Actuator.Init(ctx, &task) // 初始化任务
	if err != nil {
		return fmt.Errorf("task init failed: %v", err)
	}
	return s.Container.AddTask(ctx, *newTask)
}

// FinshedTasks 返回的完成的任务的 channel
func (s *TaskScheduler) FinshedTasks() chan *Task {
	return s.finshedTask
}

// StopTask 停止一个任务
func (s *TaskScheduler) StopTask(ctx context.Context, ftask *Task) error {
	ftask, err := s.Container.ToStopStatus(ctx, ftask)
	if err != nil {
		return err
	}
	return s.Actuator.Stop(ctx, ftask)
}

// Close 停止调度
func (s *TaskScheduler) Close() {
	if s.config.EnableFinshedTaskList {
		close(s.finshedTask)
	}
}

func (s *TaskScheduler) start() {
	go s.schedulerTask()
	go s.updateTaskStatus()
}

func (s *TaskScheduler) schedulerTask() {
	if s.config.ScanInterval == 0 {
		for {
			select {
			case <-s.ctx.Done():
				return
			default:
				s.scheduleOnce(s.ctx)
			}
		}
	} else {
		ticker := time.NewTicker(s.config.ScanInterval)
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				s.scheduleOnce(s.ctx)
			}
		}
	}
}

func (s *TaskScheduler) scheduleOnce(ctx context.Context) {
	runningCount, err := s.Container.GetRunningTaskCount(ctx)
	if err != nil {
		return
	}
	if runningCount >= s.config.TaskLimit {
		return
	}
	waitTasks, err := s.Container.GetWaitingTask(ctx, s.config.TaskLimit-runningCount)
	if err != nil {
		if err != nil {
			return
		}
	}
	wg := sync.WaitGroup{}
	for i := range waitTasks {
		task := waitTasks[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			newTask, ignore, err := s.Actuator.Start(ctx, &task)
			if err != nil {
				if !ignore {
					s.Container.ToFailedStatus(ctx, newTask, err)
				}
				return
			}
			if count, err := s.Container.GetRunningTaskCount(ctx); err == nil && count >= s.config.TaskLimit {
				// 多调度器可能出现的问题，超过任务数量限制，取消当前任务调度
				s.Actuator.Stop(ctx, newTask)
				return
			}
			_, err = s.Container.ToRunningStatus(ctx, newTask)
			if err != nil {
				s.Actuator.Stop(ctx, newTask)
				return
			}

		}()
	}
	wg.Wait()
}

func (s *TaskScheduler) updateTaskStatus() {
	if s.config.ScanInterval == 0 {
		for {
			select {
			case <-s.ctx.Done():
				return
			default:
				s.updateOnce(s.ctx)
			}
		}
	} else {
		ticker := time.NewTicker(s.config.ScanInterval)
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:

				s.updateOnce(s.ctx)
			}
		}
	}
}

func (s *TaskScheduler) finshed(ctx context.Context, task *Task) {
	// 添加到完成的任务 channel
	task.TaskEnbTime = time.Now()

	if s.config.EnableFinshedTaskList {
		c := time.NewTimer(50 * time.Millisecond)
		retryCount := 0
		select {
		case s.finshedTask <- task:
			return
		case <-c.C:
			// 因为缓存满了，导致加入不进去，chan 弹出一个元素
			// 最多超时三次
			if retryCount >= 3 {
				return
			}
			c.Reset(0)
			select {
			case <-s.finshedTask:
				break
			case <-c.C:
				break
			}
			c.Reset(0)
			retryCount++
		}
	}
}

func (s *TaskScheduler) failed(ctx context.Context, task *Task, err error) (*Task, error) {
	// 任务失败
	log.Println(task)
	newtask, err := s.Container.ToFailedStatus(ctx, task, err)
	log.Println(newtask)
	if err == nil {
		s.finshed(ctx, newtask)
	}
	return newtask, err
}

func (s *TaskScheduler) success(ctx context.Context, task *Task) (*Task, error) {
	// 任务成功
	newtask, err := s.Container.ToSuccessStatus(ctx, task)
	if err != nil {
		newtask, err = s.failed(ctx, newtask, err)
	} else {
		s.finshed(ctx, newtask)
	}
	return newtask, err
}

func (s *TaskScheduler) updateOnce(ctx context.Context) {

	runingTasks, err := s.Container.GetRunningTask(ctx)
	if err != nil {
		return
	}
	status, err := s.Actuator.GetAsyncTaskStatus(ctx, runingTasks)
	if err != nil {
		return
	}
	if len(status) != len(runingTasks) {
		log.Printf("get async task status result lentgh(%d) not euqal input length(%d)\n", len(status), len(runingTasks))
		return
	}
	wg := sync.WaitGroup{}
	for i := range runingTasks {
		task := runingTasks[i]
		st := status[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			if st.TaskStatus == TASK_STATUS_FAILED {
				s.failed(ctx, &task, st.FailedReason)
			} else if st.TaskStatus == TASK_STATUS_SUCCESS {
				newtask, err := s.Container.ToExportStatus(ctx, &task)
				if err != nil {
					s.failed(ctx, newtask, err)
					return
				}
				go func() {
					// 先从执行器获取任务执行结果
					data, err := s.Actuator.GetOutput(ctx, newtask)
					if err != nil {
						s.failed(ctx, newtask, err)
						return
					}
					// 保存任务结果
					if err := s.Container.SaveData(ctx, newtask, data); err != nil {
						s.failed(ctx, newtask, err)
						return
					}
					s.success(ctx, newtask)
				}()
			} else if st.TaskStatus == TASK_STATUS_RUNNING {
				if s.config.TaskTimeout > 0 && task.TaskStartTime.Add(s.config.TaskTimeout).Before(time.Now()) {
					// 任务超时
					newTask, err := s.failed(ctx, &task, fmt.Errorf("任务%v超时", s.config.TaskTimeout))
					if err == nil {
						s.Actuator.Stop(ctx, newTask)
					}
					return
				}
				s.Container.UpdateRunningTaskStatus(ctx, &task, st)
			}
		}()
	}
	wg.Wait()
}
