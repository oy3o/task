package task

import (
	"context"
	"fmt"
)

// Future 是一个轻量级的句柄，只持有结果和信号通道
// 相比 Promise，它没有锁，没有回调链，内存占用极小
type Future[T any] struct {
	val  T
	err  error
	done chan struct{}
}

// Get 阻塞直到任务完成。
// 吸收了 Promise 的 Await 体验，但更符合 Go 的 select 习惯。
func (f *Future[T]) Get(ctx context.Context) (T, error) {
	select {
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	case <-f.done:
		return f.val, f.err
	}
}

// Submit 泛型提交函数。
// 这是对 Runner 的扩展，不修改 Runner 内部结构，只在应用层封装。
func Submit[T any](r *Runner, fn func(ctx context.Context) (T, error)) (*Future[T], error) {
	// 1. 预分配 Future，channel 容量为 0 (同步) 或 1 均可，这里用 0 省内存
	f := &Future[T]{
		done: make(chan struct{}),
	}

	// 2. 包装任务
	taskFunc := func(ctx context.Context) {
		// 确保无论 panic 还是正常退出，done 都会关闭
		defer close(f.done)

		// 捕获 panic，将其转化为 error 返回给 Future 的调用者
		// 同时重新 panic，让 Runner 的 Worker 也能感知到并记录指标(TotalPanics)
		defer func() {
			if p := recover(); p != nil {
				f.err = fmt.Errorf("task panic: %v", p)
				panic(p)
			}
		}()

		f.val, f.err = fn(ctx)
	}

	// 3. 提交给 Worker Pool
	if err := r.Submit(taskFunc); err != nil {
		// 队列满了或 Runner 已关闭
		return nil, err
	}

	return f, nil
}

// Join 等待所有 Future 完成，并返回结果切片。
// 只要有一个 Future 报错，它会立即返回该错误（Fail Fast），但不会取消其他正在运行的任务。
// 类似 JS 的 Promise.all
func Join[T any](ctx context.Context, futures ...*Future[T]) ([]T, error) {
	results := make([]T, len(futures))

	// 优化：如果数量少，直接串行检查 channel 可能比 select 开销更低
	// 但为了支持 Context 取消，我们需要处理超时
	for i, f := range futures {
		val, err := f.Get(ctx)
		if err != nil {
			return nil, err
		}
		results[i] = val
	}

	return results, nil
}
