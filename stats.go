package task

// Stats 包含 Runner 的运行时快照
type Stats struct {
	// 配置参数
	MaxWorkers int `json:"max_workers"`
	QueueSize  int `json:"queue_size"`

	// 实时状态
	ActiveWorkers int64 `json:"active_workers"` // 当前正在执行任务的 Worker 数
	QueuedTasks   int   `json:"queued_tasks"`   // 当前排队等待的任务数

	// 累积计数 (自启动以来)
	TotalProcessed uint64 `json:"total_processed"` // 成功处理的任务数
	TotalPanics    uint64 `json:"total_panics"`    // 发生的 Panic 总数
	TotalRefused   uint64 `json:"total_refused"`   // 因队列满被拒绝的任务数
}

// Stats 获取当前 Runner 的状态快照
func (r *Runner) Stats() Stats {
	return Stats{
		MaxWorkers:     r.cfg.maxWorkers,
		QueueSize:      r.cfg.queueSize,
		ActiveWorkers:  r.activeWorkers.Load(),
		QueuedTasks:    len(r.taskChan), // len 是线程安全的近似值
		TotalProcessed: r.statsProcessed.Load(),
		TotalPanics:    r.statsPanics.Load(),
		TotalRefused:   r.statsRefused.Load(),
	}
}
