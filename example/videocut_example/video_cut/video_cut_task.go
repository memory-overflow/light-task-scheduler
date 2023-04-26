package videocut

// VideoCutTask 视频裁剪任务结构
type VideoCutTask struct {
	TaskId                   string
	CutStartTime, CutEndTime float32
	InputVideo               string
}
