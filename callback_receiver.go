package lighttaskscheduler

import "context"

// CallbackReceiver 任务状态回调接收器
type CallbackReceiver interface {
	GetCallbackChannel(ctx context.Context) (taskChannel chan Task)
}
