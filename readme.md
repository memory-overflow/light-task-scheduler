# 轻量级任务调度框架

## 框架设计
框架对整个任务调度的过程进行4种抽象。

1. [任务容器](https://github.com/memory-overflow/light-task-scheduler/blob/develop/task_container.go)——定义任务如何进行存取和状态流转。
2. [任务执行器](https://github.com/memory-overflow/light-task-scheduler/blob/develop/task_actuator.go)——定义真正执行任务的逻辑。
3. [任务数据持久化](https://github.com/memory-overflow/light-task-scheduler/blob/develop/taskdata_persistencer.go)——定义任务数据如果持久化的存储。
4. [任务调度器](https://github.com/memory-overflow/light-task-scheduler/blob/develop/task_scheduler.go)——定义实现任务调度的流程。

其中，任务容器、任务执行器、数据持久化可能和业务相关、所以用一系列的接口(interface)来抽象，开发者根据自己的业务实现接口。任务调度流程比较固定，由框架实现。

<img width="" src="/uploads/B6EC16A926104FCB938F857DE54BD5D0/image.png" alt="image.png" />


任务调度架构如下：

<img width="" src="/uploads/6CEAD9A8FA7246F6B001B98BBC00B0CB/image.png" alt="image.png" />


主要分成三个主线程

1. 调度线程：负责对等待中的任务进行限频调度。

2. 轮询维护任务状态线程：主要负责感知运行中的任务执行情况，包含超时控制、重试、状态转移，以及执行成功后数据持久化过程。

3. 回调处理线程：任务状态更新支持回调模式。

其中轮询维护任务状态和回调处理状态可以选择其一、或者同时打开。

整体的设计思想是通过抽象实现调度流程的共享，开发者只需要业务关注的部分。

## 任务容器分类

任务可以分成临时任务和持久化任务。

1. 临时任务——比如存储在内存队列里面的任务，执行完成以后，或者服务宕机、重启以后，任务相关的数据消失。
2. 可持久化任务——任务记录持久化存储，支持查询、修改和删除。

根据是否可持久化，继续对任务容器抽象，分成两类任务容器：

- [MemeoryContainer](https://github.com/memory-overflow/light-task-scheduler/blob/develop/container/memory_container/memory_container.go)——内存型任务容器，优点：可以快读快写，缺点：不可持久化。MemeoryContainer 实际上是可以和业务无关的，所以框架预置了三种MemeoryContainer——[queueContainer](https://github.com/memory-overflow/light-task-scheduler/blob/develop/container/memory_container/queue_container.go),[orderedMapContainer](https://github.com/memory-overflow/light-task-scheduler/blob/develop/container/memory_container/orderedmap_container.go),[redisContainer](https://github.com/memory-overflow/light-task-scheduler/blob/develop/container/memory_container/redis_container.go)。

  - [queueContainer](https://github.com/memory-overflow/light-task-scheduler/blob/develop/container/memory_container/queue_container.go)：queueContainer 队列型容器，任务无状态，无优先级，先进先出，任务数据，多进程数据无法共享数据

  - [orderedMapContainer](https://github.com/memory-overflow/light-task-scheduler/blob/develop/container/memory_container/orderedmap_container.go)：[OrderedMap](https://github.com/memory-overflow/go-orderedmap/blob/main/ordered_map.go) 作为容器，支持任务优先级，多进程数据无法共享数据

  - [redisContainer](https://github.com/memory-overflow/light-task-scheduler/blob/develop/container/memory_container/redis_container.go)：redis 作为容器，支持任务优先级，并且可以多进程，多副本共享数据

- [PersistContainer](https://github.com/memory-overflow/light-task-scheduler/blob/main/container/persist_container/persist_container.go)——可持久化任务容器，优点：可持久化存储，缺点：依赖db、需要扫描表，对 db 压力比较大。开发者可以参考[exampleSQLContainer](https://github.com/memory-overflow/light-task-scheduler/blob/develop/example/videocut_example/video_cut/example_sql_container.go) 实现自己的 SQLContainer，修改数据表的结构。

由于 MemeoryContainer 和 PersistContainer 各有优缺点，如果可以组合两种容器，生成一种新的任务容器[combinationContainer](https://github.com/memory-overflow/light-task-scheduler/blob/develop/upload/container.png)，既能够通过内存实现快写快读，又能够通过DB实现可持久化。

<img width="" src="/uploads/5DAE554F938A4C6FB89098EB9125B6D9/image.png" alt="image.png" />


## Usage

### 构建任务调度器

```go
func MakeScheduler(
	container TaskContainer,
	actuator TaskActuator,
	persistencer TaskdataPersistencer,
	config Config) (*TaskScheduler, error)
```
通过任务容器、执行器、持久化器和任务配置构建一个任务调度器，构建完后自动开始调度。其中持久化器可以为空，
[任务配置](https://github.com/memory-overflow/light-task-scheduler/blob/develop/task_scheduler.go#L16)
可以配置是否需要回调，如果需要回调，需要配置一个自定义的
[回调器](https://github.com/memory-overflow/light-task-scheduler/blob/develop/callback_receiver.go)。

### 函数执行器
框架预制了[函数执行器](https://github.com/memory-overflow/light-task-scheduler/blob/develop/actuator/function_actuator.go)，借助函数执行器，可以轻松实现函数调度。

使用框架预制的队列容器和函数执行器可以轻松实现一个函数的调度。参考 [a+b example](https://github.com/memory-overflow/light-task-scheduler/blob/develop/example/add_example/main.go)。



### Example: 使用内存容器实现视频裁剪异步任务调度
本例子演示如何用本框架实现一个持久化的任务调度系统。包含一个简单的 web 管理界面。

首先实现一个异步裁剪的微服务，一共四个接口，需要先安装`ffmpeg`命令
1. /VideoCut, 输入原视频, 裁剪开始时间，结束时间，返回 taskId。
2. /Status，输入 taskId, 返回该任务的状态，是否已经完成，或者失败。
3. /GetOutputVideo, 如果任务已经完成，输入 TaskId，返回已经完成的视频路径结果。
4. /Stop, 如果任务执行时间过长，可以支持停止。

服务代码参考 [video_cut_service.go](https://github.com/memory-overflow/light-task-scheduler/blob/develop/example/videocut_example/video_cut/video_cut_service.go)。

现在我们通过本任务调度框架实现一个对裁剪任务进行调度系统，可以控制任务并发数，和任务超时时间。并且按照队列依次调度。

#### 定义视频裁剪任务
```go
// VideoCutTask 视频裁剪任务结构
type VideoCutTask struct {
	TaskId                   string
	CutStartTime, CutEndTime float32
	InputVideo               string
}
```

#### 实现视频裁剪任务执行器

实现一个视频裁剪任务的执行器，执行器实际上就是调用视频裁剪微服务的 API 接口，执行器的实现参考[video_cut_actuator.go](https://github.com/memory-overflow/light-task-scheduler/blob/develop/example/videocut_example/video_cut/video_cut_actuator.go)。

#### 实现视频裁剪任务容器
首先实现一个 [sql 任务容器](https://github.com/memory-overflow/light-task-scheduler/blob/develop/example/videocut_example/video_cut/example_sql_container.go)，从数据库存取任务，然后和队列容器组合一起成为一个组合容器作为[裁剪任务的容器](https://github.com/memory-overflow/light-task-scheduler/blob/develop/example/videocut_example/main_web/main.go#L23)。

#### 实现数据持久化
这里为了连接 db 方便，把任务容器和持久化合并到了一起。
参考代码[DataPersistence](https://github.com/memory-overflow/light-task-scheduler/blob/develop/example/videocut_example/video_cut/example_sql_container.go#L387)。


#### 构建调度器
参考代码[main.go](https://github.com/memory-overflow/light-task-scheduler/blob/develop/example/videocut_example/main_web/main.go#L26)。

#### 实现简单管理接口的 web 页面
参考代码 [web.go](https://github.com/memory-overflow/light-task-scheduler/blob/develop/example/videocut_example/video_cut/web.go)。

#### 启动服务
执行 `go run example/videocut_example/main_web/main.go` 启动服务，然后在浏览器输入`http://127.0.0.1:8000/html`即可进入管理界面。