package task

import "context"

// ErrorHandler 用于处理任务执行过程中的 Panic 或错误。
// ctx: 发生错误时的上下文
// p: recover() 捕获的对象 (可能为 nil)
type ErrorHandler func(ctx context.Context, p any)

type config struct {
	maxWorkers int
	queueSize  int
	errHandler ErrorHandler
}

// Option 定义配置修改函数
type Option func(*config)

// DefaultConfig 返回默认配置
func DefaultConfig() *config {
	return &config{
		maxWorkers: 10,   // 默认 10 个并发
		queueSize:  1000, // 默认缓冲 1000 个任务
		errHandler: nil,  // 默认不处理（safely.go 会打印基础日志）
	}
}

// WithMaxWorkers 设置最大并发 Worker 数。
func WithMaxWorkers(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.maxWorkers = n
		}
	}
}

// WithQueueSize 设置等待队列的长度。
func WithQueueSize(n int) Option {
	return func(c *config) {
		if n >= 0 {
			c.queueSize = n
		}
	}
}

// WithErrorHandler 设置 Panic 处理回调。
// 建议在此处集成 zap/zerolog 等日志库。
func WithErrorHandler(h ErrorHandler) Option {
	return func(c *config) {
		c.errHandler = h
	}
}
