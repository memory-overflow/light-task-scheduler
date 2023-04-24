package persistcontainer

import lighttaskscheduler "github.com/memory-overflow/light-task-scheduler"

// PersistContainer 可持久化任务容器，优点：可持久化存储，
// 缺点：依赖db、需要扫描表获取任务队列，对 db 压力比较大。
type PersistContainer lighttaskscheduler.TaskContainer
