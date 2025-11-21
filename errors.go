package task

import "errors"

var (
	// ErrQueueFull 表示任务队列已满，无法接受新任务。
	// 调用者应该处理此错误（例如：返回 503，或者降级）。
	ErrQueueFull = errors.New("task: queue is full")

	// ErrRunnerClosed 表示 Runner 已经停止或正在停止，不再接受新任务。
	ErrRunnerClosed = errors.New("task: runner is closed")
)
