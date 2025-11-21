package task

import (
	"context"
	"fmt"
	"runtime/debug"
)

// Safely 执行一个函数，并捕获可能发生的 Panic。
// fn: 接收上下文的任务函数
// handler: 发生 Panic 时的回调
func Safely(ctx context.Context, fn func(context.Context), handler ErrorHandler) {
	defer func() {
		if r := recover(); r != nil {
			if handler != nil {
				handler(ctx, r)
			} else {
				// 兜底日志
				fmt.Printf("task: panic recovered: %v\n%s\n", r, debug.Stack())
			}
		}
	}()
	// 将上下文传递给具体执行的函数
	fn(ctx)
}

// Go 是 go func() 的安全替代品。
// 它启动一个新的协程并执行 fn，自带 Panic 保护。
func Go(ctx context.Context, fn func(context.Context), handler ErrorHandler) {
	go Safely(ctx, fn, handler)
}
