package lighttaskscheduler

import (
	"context"
	"errors"
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
	TaskTimeout  time.Duration
}

// TaskScheduler 任务调度器，通过对任务容器和任务执行器的操作，实现任务调度
type TaskScheduler struct {
	// Container 配置的任务容器
	Container TaskContainer
	// Actuator 配置的任务执行器
	Actuator TaskActuator

	config Config
	ctx    context.Context
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
	go scheduler.start()
	return scheduler
}

// AddTask 添加一个任务，需要把任务转换成 lighttaskscheduler.Task 的通用形式
// 注意一定要配置一个唯一的任务 id 标识
func (s *TaskScheduler) AddTask(ctx context.Context, task Task) error {
	return s.Container.AddTask(ctx, task)
}

// Close 停止调度
func (s *TaskScheduler) Close() {
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
				s.Container.ToFailedStatus(ctx, &task, st.FailedReason)
			} else if st.TaskStatus == TASK_STATUS_SUCCESS {
				newtask, err := s.Container.ToExportStatus(ctx, &task)
				if err != nil {
					return
				}
				go func() {
					// 导出结果可能比较耗时，异步导出
					if err := s.Actuator.ExportOutput(ctx, newtask); err != nil {
						s.Container.ToFailedStatus(ctx, newtask, err)
						return
					}
					if _, err := s.Container.ToSuccessStatus(ctx, newtask); err != nil {
						s.Actuator.Delete(ctx, newtask) // 任务更新失败，执行器删除 ExportOutput 生成的相关资源
						s.Container.ToFailedStatus(ctx, newtask, err)
					}
				}()
			} else if st.TaskStatus == TASK_STATUS_RUNNING {
				if s.config.TaskTimeout > 0 && task.TaskStartTime.Add(s.config.TaskTimeout).Before(time.Now()) {
					// 任务超时
					newTask, err := s.Container.ToFailedStatus(ctx, &task, errors.New("任务超时"))
					if err != nil {
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
