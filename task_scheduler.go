package lighttaskscheduler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"sync"
	"time"

	stlextension "github.com/memory-overflow/go-orderedmap"
)

// Config 配置
type Config struct {
	// 任务执行超时时间，超过该时间后，任务将被强制结束，并且视为任务失败
	TaskTimeout time.Duration

	// 任务并发限制
	TaskLimit int32

	// 任务失败最大尝试次数
	MaxFailedAttempts int32

	// 任务调度固定使用轮询，会定期使用任务容器的接口获取执行中的任务数和任务等待队列中的任务
	// SchedulingPollInterval 用来配置该定期轮询的时间周期
	// 根据任务容器配置合理的值，比如 db 任务容器，配置合理的轮询间隔，避免对 db 压力过大
	SchedulingPollInterval time.Duration

	// 任务状态维护默认使用轮询，定期通过执行器接口获取执行中的任务状态，然后对任务状态进行更新
	// 如果 DisableStatePoll 设置为 true，将关闭轮询状态维护，
	// 但是必须要开启 EnableStateCallback 通过回调的方式通知任务状态
	DisableStatePoll bool

	// 在 DisableStatePoll 为 false 的情况下生效，任务状态维护轮询的时间周期
	// 根据任务容器配置合理的值，比如 db 任务容器，配置合理的轮询间隔，避免对 db 压力过大
	StatePollInterval time.Duration

	// 是否开启任务状态回调，回调比轮询对任务状态维护具有更地的延时，能够及时更新完成的任务
	// 如果有条件，建议同时开始轮询和回调，回调可以更早的感知任务结束，
	// 轮询可以为任务回调失败或者丢失兜底，保证任务状态一定可以更新
	// 单回调模式，无法对任务进行超时感知处理
	EnableStateCallback bool

	// CallbackReceiver 任务回调接收器
	// 如果 EnableStateCallback 为 true 开启任务状态回调，必须要要配置任务回调接收器
	CallbackReceiver CallbackReceiver
	// 是否需要调度器返回已完成的任务
	// 如果为 true，需要通过
	// for finishedTask := range TaskScheduler.FinshedTasks() {
	//   ...
	// }
	// 及时取走 channel 中的数据，否则可能造成 channel 满了，任务调度阻塞
	// 默认不开启，为 false
	EnableFinshedTaskList bool
}

func (c *Config) check() error {
	if c.DisableStatePoll && !c.EnableStateCallback {
		return fmt.Errorf("unreasonable config, DisableStatePoll must with set EnableStateCallback true")
	}

	if c.EnableStateCallback && c.CallbackReceiver == nil {
		return fmt.Errorf("unreasonable config, if set EnableStateCallback true, must set CallbackReceiver")
	}
	return nil
}

type processTime struct {
	t      time.Time
	taskId string
}

// TaskScheduler 任务调度器，通过对任务容器和任务执行器的操作，实现任务调度
type TaskScheduler struct {
	// Container 配置的任务容器
	Container TaskContainer
	// Actuator 配置的任务执行器
	Actuator TaskActuator
	// Persistencer 数据持久化
	Persistencer TaskdataPersistencer

	finshedTask chan *Task // 回调给用户已完成的任务
	config      Config
	ctx         context.Context
	cancel      context.CancelFunc

	enableProcessedCheck bool
	// 记录 5 秒内处理过状态的任务，防止回调和轮询重复处理一个任务的结束状态
	processedTask              map[string]bool
	taskProcessedTime          []processTime
	bufflen, head, tail, count int
	lock                       sync.Mutex

	wg *stlextension.LimitWaitGroup
}

// MakeScheduler 新建任务调度器
// 如果不需要对任务数据此久化，persistencer 可以设置为 nil
// 调度器构建以后，自动开始任务调度
func MakeScheduler(
	container TaskContainer,
	actuator TaskActuator,
	persistencer TaskdataPersistencer,
	config Config) (*TaskScheduler, error) {
	if err := config.check(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	scheduler := &TaskScheduler{
		Container:    container,
		Actuator:     actuator,
		Persistencer: persistencer,
		config:       config,
		ctx:          ctx,
		cancel:       cancel,
		wg:           stlextension.NewLimitWaitGroup(20),
		head:         0,
		tail:         0,
		count:        0,
	}
	if config.EnableFinshedTaskList {
		scheduler.finshedTask = make(chan *Task, 10000)
	}
	go scheduler.start()
	return scheduler, nil
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
	oldStaus := ftask.TaskStatus
	ftask, err := s.Container.ToStopStatus(ctx, ftask)
	if err != nil {
		return err
	}
	if oldStaus == TASK_STATUS_RUNNING {
		s.Actuator.Stop(ctx, ftask)
	}
	return nil

}

// Close 停止调度
func (s *TaskScheduler) Close() {
	if s.config.EnableFinshedTaskList {
		close(s.finshedTask)
	}
	s.cancel()
}

func (s *TaskScheduler) checkProcessed(taskId string) bool {
	if !s.enableProcessedCheck {
		return true
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, ok := s.processedTask[taskId]; !ok {
		s.processedTask[taskId] = true
		s.taskProcessedTime[s.tail] = processTime{
			t:      time.Now(),
			taskId: taskId,
		}
		s.count++
		s.tail++
		if s.tail >= s.bufflen {
			s.tail = 0
		}
		return true
	} else {
		return false
	}
}

func (s *TaskScheduler) cleanProcessTask() {
	ticker := time.NewTicker(3 * time.Second)
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			func() {
				s.lock.Lock()
				defer func() {
					if p := recover(); p != nil {
						fmt.Printf("panic=%v stacktrace=%s\n", p, debug.Stack())
					}
					s.lock.Unlock()
				}()

				for s.count > 0 {
					if s.taskProcessedTime[s.head].t.After(time.Now().Add(-5 * time.Second)) {
						delete(s.processedTask, s.taskProcessedTime[s.head].taskId)
						s.head++
						s.count--
						if s.head > s.bufflen {
							s.bufflen = 0
						}
					} else {
						break
					}
				}

			}()

		}
	}
}

func (s *TaskScheduler) start() {
	go s.schedulerTask()

	if s.config.EnableStateCallback {
		go s.updateCallbackTask()
	}

	if !s.config.DisableStatePoll {
		go s.updateTaskStatus()
	}

	if s.config.EnableStateCallback && !s.config.DisableStatePoll {
		// 如果同时开启轮询和回调，必须要开启重复处理检测
		s.enableProcessedCheck = true
		s.processedTask = make(map[string]bool)
		s.bufflen = 10000
		s.taskProcessedTime = make([]processTime, s.bufflen)
		go s.cleanProcessTask()
	}
}

func (s *TaskScheduler) schedulerTask() {
	if s.config.SchedulingPollInterval == 0 {
		for {
			select {
			case <-s.ctx.Done():
				return
			default:
				s.scheduleOnce(s.ctx)
			}
		}
	} else {
		ticker := time.NewTicker(s.config.SchedulingPollInterval)
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
		return
	}
	wg := stlextension.NewLimitWaitGroup(20)
	for i := range waitTasks {
		task := waitTasks[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			newTask, ignore, err := s.Actuator.Start(ctx, &task)
			if err != nil {
				if !ignore {
					s.failed(s.ctx, newTask, fmt.Errorf("start task error: %v", err))
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
				s.failed(s.ctx, newTask, fmt.Errorf("taskl ToRunningStatus error: %v", err))
				return
			}

		}()
	}
	wg.Wait()
}

func (s *TaskScheduler) updateTaskStatus() {
	if s.config.StatePollInterval == 0 {
		for {
			select {
			case <-s.ctx.Done():
				return
			default:
				s.updateOnce(s.ctx)
			}
		}
	} else {
		ticker := time.NewTicker(s.config.StatePollInterval)
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

func (s *TaskScheduler) updateCallbackTask() {
	for t := range s.config.CallbackReceiver.GetCallbackChannel(s.ctx) {
		// 可能是轮询已经处理过，或者重复回调
		if !s.checkProcessed(t.TaskId) {
			continue
		}
		s.wg.Add(1)
		task := t
		go func() {
			defer s.wg.Done()
			if task.TaskStatus == TASK_STATUS_FAILED {
				// 失败可以重试
				if task.TaskAttemptsTime < s.config.MaxFailedAttempts {
					task.TaskAttemptsTime++
					newTask, _, err := s.Actuator.Start(s.ctx, &task)
					// 尝试重启失败
					if err != nil {
						resaon := fmt.Errorf("任务执行失败：%v, 并且尝试重启也失败 %v", task.FailedReason, err)
						s.failed(s.ctx, &task, resaon)
					} else {
						_, err = s.Container.ToRunningStatus(s.ctx, newTask) // 更新状态
						if err != nil {
							s.Actuator.Stop(s.ctx, newTask)
							resaon := fmt.Errorf("任务执行失败：%v, 并且尝试重启也失败 %v", task.FailedReason, err)
							s.failed(s.ctx, &task, resaon)
							return
						}
					}
				} else {
					s.failed(s.ctx, &task, errors.New(task.FailedReason))
				}
			} else if task.TaskStatus == TASK_STATUS_SUCCESS {
				s.export(s.ctx, &task)
			}
		}()
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
		s.wg.Add(1)
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer s.wg.Done()
			if st.TaskStatus == TASK_STATUS_FAILED {
				// 已经回调处理过
				if !s.checkProcessed(task.TaskId) {
					return
				}
				// 失败可以重试
				if task.TaskAttemptsTime < s.config.MaxFailedAttempts {
					task.TaskAttemptsTime++
					newTask, _, err := s.Actuator.Start(ctx, &task)
					// 尝试重启失败
					if err != nil {
						resaon := fmt.Errorf("任务执行失败：%v, 并且尝试重启也失败 %v", st.FailedReason, err)
						s.failed(ctx, &task, resaon)
					} else {
						_, err = s.Container.ToRunningStatus(ctx, newTask) // 更新状态
						if err != nil {
							s.Actuator.Stop(ctx, newTask)
							resaon := fmt.Errorf("任务执行失败：%v, 并且尝试重启也失败 %v", st.FailedReason, err)
							s.failed(ctx, &task, resaon)
							return
						}
					}
				} else {
					s.failed(ctx, &task, st.FailedReason)
				}
			} else if st.TaskStatus == TASK_STATUS_SUCCESS {
				// 已经回调处理过
				if !s.checkProcessed(task.TaskId) {
					return
				}
				s.export(ctx, &task)
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

func (s *TaskScheduler) export(ctx context.Context, task *Task) {
	if s.Persistencer != nil {
		newtask, err := s.Container.ToExportStatus(ctx, task)
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
			if err := s.Persistencer.DataPersistence(ctx, newtask, data); err != nil {
				s.failed(ctx, newtask, err)
				return
			}
			s.success(ctx, newtask)
		}()
	} else {
		s.success(ctx, task)
	}
}

func (s *TaskScheduler) failed(ctx context.Context, task *Task, err error) (*Task, error) {
	// 任务失败
	newtask, err := s.Container.ToFailedStatus(ctx, task, err)
	if err == nil {
		s.finshed(ctx, newtask)
	}
	return newtask, err
}

func (s *TaskScheduler) success(ctx context.Context, task *Task) (*Task, error) {
	// 任务成功
	newtask, err := s.Container.ToSuccessStatus(ctx, task)
	if err != nil {
		if s.Persistencer != nil {
			s.Persistencer.DeletePersistenceData(ctx, task)
		}
		newtask, err = s.failed(ctx, newtask, err)
	} else {
		s.finshed(ctx, newtask)
	}
	return newtask, err
}
