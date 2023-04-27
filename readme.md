- [轻量级任务调度框架](#轻量级任务调度框架)
	- [框架的设计思想和背景](#框架的设计思想和背景)
		- [任务系统的整体设计](#任务系统的整体设计)
		- [任务调度框架](#任务调度框架)
		- [任务容器分类](#任务容器分类)
			- [1. 同步任务和异步任务](#1-同步任务和异步任务)
			- [2. 可持久任务和不可持久化任务](#2-可持久任务和不可持久化任务)
	- [Usage](#usage)
		- [使用内存容器实现视频裁剪异步任务调度](#使用内存容器实现视频裁剪异步任务调度)
			- [定义视频裁剪任务](#定义视频裁剪任务)
			- [实现视频裁剪任务执行器](#实现视频裁剪任务执行器)
			- [实现视频裁剪任务容器](#实现视频裁剪任务容器)
			- [实现调度](#实现调度)

# 轻量级任务调度框架

## 框架的设计思想和背景
业务后台开发，经常会遇到很多并发低，处理耗时长的限频任务，比如流媒体行业的视频转码、裁剪。这些任务的调度流程整体上没有差别，但是细节上又会有很多差异，以至于每新增一个任务类型，都要把调度部分的代码 copy 一份后修修改改。随着任务类型的越来越多，copy 代码的方式效率低下，且容易改出问题，这时候需要一个任务调度框架，既能够统一任务调度的流程，也能够适应差异。

### 任务系统的整体设计
一个完善的任务系统包含任务管理模块和任务调度模块。

任务管理负责任务的创建、启动、停止、删除、拉取状态、拉取分析数据等操作，多类型的任务是可以共用的。

任务调度负责任务限频、具体的业务执行、结果处理等流程，不同任务的类型的调度模块无法共用。

![image](https://user-images.githubusercontent.com/15645203/210738936-74ac3abf-8fc7-4570-a440-0a071b87daa5.png)

### 任务调度框架
作者一直想实现一个通用的任务管理框架，但是长时间陷入了应该如何存储任务的纠结境地：

1. 存 DB 可以满足异步任务需要可持久化，有状态的需求，但是需要轮询DB，性能上满足不了大流量场景。

2. 用 Kafka、内存等队列的方式存取任务，性能上很好，但是任务无状态，不能满足任务有状态的场景。

经过长时间的对比不同类型任务的执行流程，发现这些任务在大的流程上没有区别，细节上差别很大。最终，发现一个任务系统可以抽象成为三部分

1. 任务容器——用来存储任务相关数据。
2. 任务执行器——真正的执行任务的逻辑。
3. 任务调度器——对任务容器和任务执行器进行一些逻辑操作和逻辑配合，完成整体的任务调度流程。

其中，容器和执行器和业务相关、可以用一系列的接口(interface)来抽象，开发者根据自己的业务实现接口。任务调度流程比较固定，可以由框架实现。

![image](https://user-images.githubusercontent.com/15645203/210739259-86ef6480-097f-4189-98ac-3fe670dbe40b.png)

基于抽象的容器和执行器，固定的任务调度和执行的流程如下图

![image](https://user-images.githubusercontent.com/15645203/210739392-637269f6-a009-4345-92b7-8e8b92d1b3a5.png)

主要分成两个主线程

1. 维护任务状态线程：主要负责感知运行中的任务执行情况，状态的转移，以及运行成功后结果的导出过程（资源转存、数据落库等）。

2. 调度线程：负责对等待中的任务进行限频和调度。

总结下来，整体的设计思想是通过抽象出任务容器和任务执行器接口来实现调度流程的共享。

开发者只需要实现自己的任务容器接口和任务执行器接口，用任务容器和任务执行器创建任务调度器，即可轻易的实现任务调度。该框架可以支持多副本的调度器，如果使用关系型 db 作为容器，注意使用 db 的原子性防止任务重复调度。

### 任务容器分类

任务根据不同的维度，任务可以分成

#### 1. 同步任务和异步任务
本框架主要实现了任务异步化，可以轻易的通过执行器的实现把同步任务转换成异步任务。注意：实现同步任务执行器的时候，不要阻塞 Start 方法，而是在单独的协程执行任务。

#### 2. 可持久任务和不可持久化任务
任务的是否可持久化，通俗来说，就是任务执行完成以后，是否还能查询到任务相关的信息和记录。

1. 不可持久化任务——比如存储在内存队列里面的任务，执行完成以后，或者服务宕机、重启以后，任务相关的数据消失，无迹可寻。
2. 可持久化任务——一般是存储在 DB 里面的任务。

根据是否可持久化，我们继续对任务容器抽象，分成两类任务容器：

- [MemeoryContainer](https://github.com/memory-overflow/light-task-scheduler/blob/main/container/memory_container/memory_container.go)——内存型任务容器，优点：可以快读快写，缺点：不可持久化。MemeoryContainer 实际上是可以和业务无关的，所以框架预置了三种MemeoryContainer——[queueContainer](https://github.com/memory-overflow/light-task-scheduler/blob/main/container/memory_container/queue_container.go),[orderedMapContainer](https://github.com/memory-overflow/light-task-scheduler/blob/main/container/memory_container/orderedmap_container.go),[redisContainer](https://github.com/memory-overflow/light-task-scheduler/blob/main/container/memory_container/redis_container.go)。

  - [queueContainer](https://github.com/memory-overflow/light-task-scheduler/blob/main/container/memory_container/queue_container.go)：queueContainer 队列型容器，任务无状态，无优先级，先进先出，任务数据，多进程数据无法共享数据

  - [orderedMapContainer](https://github.com/memory-overflow/light-task-scheduler/blob/main/container/memory_container/orderedmap_container.go)：[OrderedMap](https://github.com/memory-overflow/go-orderedmap/blob/main/ordered_map.go) 作为容器，支持任务优先级，多进程数据无法共享数据

  - [redisContainer](https://github.com/memory-overflow/light-task-scheduler/blob/main/container/memory_container/redis_container.go)：redis 作为容器，支持任务优先级，并且可以多进程，多副本共享数据

- [PersistContainer](https://github.com/memory-overflow/light-task-scheduler/blob/main/container/persist_container/persist_container.go)——可持久化任务容器，优点：可持久化存储，缺点：依赖db、需要扫描表，对 db 压力比较大。开发者可以参考[exampleSQLContainer](https://github.com/memory-overflow/light-task-scheduler/blob/main/container/persist_container/example_sql_container.go) 实现自己的 SQLContainer，修改数据表的结构。

由于 MemeoryContainer 和 PersistContainer 各有优缺点，如果可以组合两种容器，生成一种新的任务容器[combinationContainer](https://github.com/memory-overflow/light-task-scheduler/blob/main/container/combination_container.go)，既能够通过内存实现快写快读，又能够通过DB实现可持久化。
![image](https://user-images.githubusercontent.com/15645203/210742391-ae2c60ac-9f19-4d1a-947b-634e3a3855ef.png)

## Usage

### 使用内存容器实现视频裁剪异步任务调度

首先实现一个异步裁剪的微服务，一共四个接口，需要先安装`ffmpeg`命令
1. /VideoCut, 输入原视频, 裁剪开始时间，结束时间，返回 taskId。
2. /Status，输入 taskId, 返回该任务的状态，是否已经完成，或者失败。
3. /GetOutputVideo, 如果任务已经完成，输入 TaskId，返回已经完成的视频路径结果。
4. /Stop, 如果任务执行时间过长，可以支持停止。

服务代码参考 [video_cut_service.go](https://github.com/memory-overflow/light-task-scheduler/blob/dev/demo-video-cut/example/videocut_example/video_cut/video_cut_service.go)。

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
实现一个视频裁剪任务的执行器，执行器实际上就是调用视频裁剪微服务的 API 接口。执行器的实现参考[video_cut_actuator.go](https://github.com/memory-overflow/light-task-scheduler/blob/dev/demo-video-cut/example/videocut_example/video_cut/video_cut_actuator.go), 这里对任务结果只是输出到 stdout 展示，不做后续更多处理了。

#### 实现视频裁剪任务容器
这里，我们简单的直接使用队列来作为任务容器，所以可以直接用框架预置的 queueContainer 作为任务容器，无需单独实现。

#### 实现调度
参考代码[main.go](https://github.com/memory-overflow/light-task-scheduler/blob/dev/demo-video-cut/example/videocut_example/main.go)，`go run main.go xxx.mp4` 可以执行 demo（需要安装 ffmpeg 命令）。

```go
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
					// 这里的任务 ID 是为了调度框架方便标识唯一任务的ID,
					// 和微服务的任务ID不同，是映射关系
					TaskId: strconv.Itoa(i), 
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
<<<<<<< HEAD
```
=======

```
>>>>>>> 7a438e32d39d825a30aa89e269399dbb9922cc1f
