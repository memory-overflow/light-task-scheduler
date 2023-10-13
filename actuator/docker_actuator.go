// 封装好的 docker 执行器

package actuator

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	framework "github.com/memory-overflow/light-task-scheduler"
)

// DockerTask docker 任务
type DockerTask struct {
	Image         string   // 镜像
	Cmd           []string // 容器执行命令
	ContainerName string   // 容器名，不能重复

	// 资源限制
	MemoryLimit int64 // bytes
	CpuPercent  int   // cpu 占用百分比, 比如暂用 2 核就是 200

	// 可选配置
	ExposedPorts    []string          // 暴露端口
	Env             []string          // 环境变量
	WorkingDir      string            // 工作目录
	NetworkDisabled bool              // 是否关闭网络
	NetworkMode     string            // 网络模式
	Privileged      bool              // Is the container in privileged mode
	VolumeBinds     map[string]string // host 路径到容器路径的映射，类似于 -v 参数

	ContainerId string // 执行后对应的 容器 Id
}

// dockerActuator docker 执行器
type dockerActuator struct {
	initFunc        InitFunction        // 初始函数
	callbackChannel chan framework.Task // 回调队列

	runningTask sync.Map // ContainerName -> containerId 映射
	datatMap    sync.Map // ContainerName -> interface{} 映射
}

func MakeDockerActuator(initFunc InitFunction) *dockerActuator {
	return &dockerActuator{
		initFunc: initFunc,
	}
}

// SetCallbackChannel 任务配置回调 channel
func (dc *dockerActuator) SetCallbackChannel(callbackChannel chan framework.Task) {
	dc.callbackChannel = callbackChannel
}

// Init 任务在被调度前的初始化工作
func (dc *dockerActuator) Init(ctx context.Context, task *framework.Task) (
	newTask *framework.Task, err error) {
	if dc.initFunc != nil {
		return dc.initFunc(ctx, task)
	}
	return task, nil
}

// Start 执行任务
func (dc *dockerActuator) Start(ctx context.Context, ftask *framework.Task) (
	newTask *framework.Task, ignoreErr bool, err error) {
	var task DockerTask
	if v, ok := ftask.TaskItem.(DockerTask); ok {
		task = v
	} else if v, ok := ftask.TaskItem.(*DockerTask); ok {
		task = *v
	} else {
		return ftask, false, fmt.Errorf("TaskItem is not configured as DockerTask")
	}
	cli, err := dockerclient.NewClientWithOpts(dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return ftask, false, fmt.Errorf("new docker client with error: %v", err)
	}
	exposed := nat.PortSet{}
	for _, port := range task.ExposedPorts {
		exposed[nat.Port(port)] = struct{}{}
	}
	mounts := []mount.Mount{}
	for src, dst := range task.VolumeBinds {
		mounts = append(mounts, mount.Mount{
			Type:   "bind",
			Source: src,
			Target: dst,
		})
	}
	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image:        task.Image,
			Cmd:          task.Cmd,
			Env:          task.Env,
			WorkingDir:   task.WorkingDir,
			ExposedPorts: exposed,
		},
		&container.HostConfig{
			AutoRemove: true,
			Mounts:     mounts,
			Resources: container.Resources{
				Memory:    task.MemoryLimit,
				CPUPeriod: 100000,
				CPUQuota:  int64(task.CpuPercent) * 100000 / 100,
			},
		},
		nil, nil, task.ContainerName)
	if err != nil {
		return ftask, false, fmt.Errorf("create container error: %v", err)
	}

	// 启动容器
	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return ftask, false, fmt.Errorf("start container error: %v", err)
	}

	ftask.TaskStatus = framework.TASK_STATUS_RUNNING
	ftask.TaskStartTime = time.Now()
	task.ContainerId = resp.ID
	ftask.TaskItem = task
	dc.runningTask.Store(task.ContainerName, resp.ID)

	return ftask, false, nil
}

// Stop 停止任务
func (dc *dockerActuator) Stop(ctx context.Context, ftask *framework.Task) error {
	var task DockerTask
	if v, ok := ftask.TaskItem.(DockerTask); ok {
		task = v
	} else if v, ok := ftask.TaskItem.(*DockerTask); ok {
		task = *v
	} else {
		return fmt.Errorf("TaskItem is not configured as DockerTask")
	}

	cli, err := dockerclient.NewClientWithOpts(dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("new docker client error: %v", err)
	}
	if v, ok := dc.runningTask.Load(task.ContainerName); ok {
		id := v.(string)
		err := cli.ContainerRemove(ctx, id, types.ContainerRemoveOptions{Force: true})
		if err != nil {
			return fmt.Errorf("remove container with id %s error: %v", id, err)
		}
		dc.runningTask.Delete(task.ContainerName)
	}

	return nil
}

// GetAsyncTaskStatus 批量获取任务状态
func (fc *dockerActuator) GetAsyncTaskStatus(ctx context.Context, ftasks []framework.Task) (
	status []framework.AsyncTaskStatus, err error) {
	for i, ftask := range ftasks {
		var task DockerTask
		if v, ok := ftask.TaskItem.(DockerTask); ok {
			task = v
		} else if v, ok := ftask.TaskItem.(*DockerTask); ok {
			task = *v
		} else {
			return nil, fmt.Errorf("tasks[%d].TaskItem is not configured as DockerTask", i)
		}
		cli, err := dockerclient.NewClientWithOpts(dockerclient.WithAPIVersionNegotiation())
		if err != nil {
			return nil, fmt.Errorf("new docker client error: %v", err)
		}

		v, ok := fc.runningTask.Load(task.ContainerName)
		if !ok {
			status = append(status, framework.AsyncTaskStatus{
				TaskStatus:   framework.TASK_STATUS_FAILED,
				FailedReason: errors.New("同步任务未找到"),
				Progress:     float32(0.0),
			})
		} else {
			id := v.(string)
			stats, err := cli.ContainerInspect(ctx, id)
			if err != nil {
				return nil, fmt.Errorf("get container stats for id %s error: %v", id, err)
			}
			st := framework.AsyncTaskStatus{
				TaskStatus: framework.TASK_STATUS_RUNNING,
				Progress:   float32(0.0),
			}
			if stats.State.Status != "running" {
				if stats.State.ExitCode == 0 {
					st.TaskStatus = framework.TASK_STATUS_SUCCESS
				} else {
					st.TaskStatus = framework.TASK_STATUS_FAILED
					st.FailedReason = fmt.Errorf(stats.State.Error)
				}
			}
			if st.TaskStatus != framework.TASK_STATUS_RUNNING {
				fc.runningTask.Delete(ftask.TaskId) // delete task status after query if task finished
				cli.ContainerRemove(ctx, id, types.ContainerRemoveOptions{Force: true})
			}
			status = append(status, st)
		}
	}
	return status, nil
}

// GetOutput ...
func (dc *dockerActuator) GetOutput(ctx context.Context, ftask *framework.Task) (
	data interface{}, err error) {
	var task DockerTask
	if v, ok := ftask.TaskItem.(DockerTask); ok {
		task = v
	} else if v, ok := ftask.TaskItem.(*DockerTask); ok {
		task = *v
	} else {
		return nil, fmt.Errorf("TaskItem is not configured as DockerTask")
	}
	if v, ok := dc.datatMap.LoadAndDelete(task.ContainerName); ok {
		return v, nil
	} else {
		return nil, fmt.Errorf("task data not found")
	}
}
