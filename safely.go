package task

import (
	"context"
	"fmt"
	"runtime/debug"
)

// Safely 执行一个函数，并捕获可能发生的 Panic。
// fn: 接收上下文的任务函数
// handler: 发生 Panic 时的回调
// 返回值 panicked 用于精准通知调用方任务是否异常崩溃。
func Safely(ctx context.Context, fn func(context.Context), handler ErrorHandler) (panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			panicked = true // 状态转移标记
			if handler != nil {
				// 二层保护：防止 handler 自身 panic 击穿 Worker goroutine
				func() {
					defer func() {
						if hp := recover(); hp != nil {
							fmt.Printf("task: error handler panicked: %v\noriginal panic: %v\n%s\n", hp, r, debug.Stack())
						}
					}()
					handler(ctx, r)
				}()
			} else {
				fmt.Printf("task: panic recovered: %v\n%s\n", r, debug.Stack())
			}
		}
	}()

	fn(ctx)
	return false
}

// Go 是 go func() 的安全替代品。
// 它启动一个新的协程并执行 fn，自带 Panic 保护。
func Go(ctx context.Context, fn func(context.Context), handler ErrorHandler) {
	go Safely(ctx, fn, handler) // 异步执行不关心返回值，忽略即可
}
