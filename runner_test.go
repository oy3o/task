package task

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 测试基本的并发执行
func TestRunner_Concurrency(t *testing.T) {
	// 限制 2 个并发，队列 100
	r := NewRunner(WithMaxWorkers(2), WithQueueSize(100))
	r.Start(context.Background())
	defer r.Stop(context.Background())

	count := 10
	var wg sync.WaitGroup
	wg.Add(count)

	var executed atomic.Int32

	for i := 0; i < count; i++ {
		err := r.Submit(func(ctx context.Context) {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond) // 模拟耗时
			executed.Add(1)
		})
		require.NoError(t, err)
	}

	wg.Wait()
	assert.Equal(t, int32(count), executed.Load())
}

// 测试队列满时的拒绝策略
func TestRunner_QueueFull(t *testing.T) {
	// 1个 worker，1个队列容量
	// 总共只能容纳：1 (正在运行) + 1 (排队) = 2 个任务
	r := NewRunner(WithMaxWorkers(1), WithQueueSize(1))
	r.Start(context.Background())
	defer r.Stop(context.Background())

	// 阻塞 Worker 的 channel
	blocker := make(chan struct{})

	// 任务 1: 占用 Worker
	r.Submit(func(ctx context.Context) {
		<-blocker
	})

	// 确保任务 1 已经开始运行
	time.Sleep(10 * time.Millisecond)

	// 任务 2: 占用队列
	err := r.Submit(func(ctx context.Context) {})
	assert.NoError(t, err, "Should accept task into queue")

	// 任务 3: 应该被拒绝
	err = r.Submit(func(ctx context.Context) {})
	assert.Equal(t, ErrQueueFull, err, "Should reject when queue is full")

	// 释放 Worker
	close(blocker)
}

// 测试 SubmitAndWait (同步等待)
func TestRunner_SubmitAndWait(t *testing.T) {
	r := NewRunner()
	r.Start(context.Background())
	defer r.Stop(context.Background())

	t.Run("Success", func(t *testing.T) {
		err := r.SubmitAndWait(context.Background(), func(ctx context.Context) {
			// do nothing
		})
		assert.NoError(t, err)
	})

	t.Run("Context Cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		// 启动一个耗时任务
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		err := r.SubmitAndWait(ctx, func(innerCtx context.Context) {
			time.Sleep(100 * time.Millisecond)
		})

		// 因为外层 Context 取消了，所以 SubmitAndWait 应该返回 ctx.Err()
		assert.ErrorIs(t, err, context.Canceled)
	})
}

// 测试优雅关闭 (Graceful Shutdown)
func TestRunner_Stop_Drain(t *testing.T) {
	r := NewRunner(WithMaxWorkers(2))
	r.Start(context.Background())

	var completed atomic.Int32

	// 提交 5 个任务，每个耗时 50ms
	for i := 0; i < 5; i++ {
		r.Submit(func(ctx context.Context) {
			time.Sleep(50 * time.Millisecond)
			completed.Add(1)
		})
	}

	// 立即停止
	startStop := time.Now()
	err := r.Stop(context.Background())
	duration := time.Since(startStop)

	assert.NoError(t, err)
	assert.Equal(t, int32(5), completed.Load(), "All tasks should be completed")
	// 耗时应该至少是 50ms * 3轮 (2并发: 2+2+1) = 150ms 左右
	assert.True(t, duration >= 50*time.Millisecond, "Stop should wait for tasks")

	// 停止后提交应该报错
	err = r.Submit(func(ctx context.Context) {})
	assert.Equal(t, ErrRunnerClosed, err)
}

// 测试 Stop 超时强制退出
func TestRunner_Stop_Timeout(t *testing.T) {
	r := NewRunner(WithMaxWorkers(1))
	r.Start(context.Background())

	// 提交一个死循环任务
	r.Submit(func(ctx context.Context) {
		select {} // block forever
	})

	// 尝试停止，超时设置为 100ms
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := r.Stop(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// 测试竞态条件：高并发提交时调用 Stop
// 这个测试验证了 sync.RWMutex 是否有效防止了 "send on closed channel" panic
func TestRunner_Race_Submit_Stop(t *testing.T) {
	r := NewRunner(WithMaxWorkers(10), WithQueueSize(1000))
	r.Start(context.Background())

	var wg sync.WaitGroup
	workers := 50
	loops := 100

	// 启动大量并发提交者
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < loops; j++ {
				err := r.Submit(func(ctx context.Context) {
					// do nothing
				})
				// 这里我们不关心是否成功，只关心是否 panic
				if err != nil {
					if err != ErrRunnerClosed && err != ErrQueueFull {
						t.Errorf("Unexpected error: %v", err)
					}
				}
			}
		}()
	}

	// 让提交者跑一会
	time.Sleep(5 * time.Millisecond)

	// 在高并发提交过程中停止 Runner
	r.Stop(context.Background())

	wg.Wait()
}

// 测试：通过 Stop() 取消任务上下文，使得阻塞任务能立即退出
func TestRunner_Stop_WithContextCancellation(t *testing.T) {
	r := NewRunner(WithMaxWorkers(1))
	r.Start(context.Background())

	taskExited := make(chan struct{})

	// 提交一个监听 ctx.Done() 的阻塞任务
	r.Submit(func(ctx context.Context) {
		select {
		case <-ctx.Done():
			// 收到取消信号，正常退出
			close(taskExited)
		case <-time.After(5 * time.Second):
			t.Error("Task should have been cancelled")
		}
	})

	// 确保任务已经开始运行
	time.Sleep(50 * time.Millisecond)

	// 调用 Stop，这应该触发 context cancel
	// 我们设置一个很长的超时，如果 context 机制不工作，Stop 就会一直等到 task 自然超时（5s），这会导致测试变慢或失败
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := r.Stop(ctx)
	assert.NoError(t, err, "Stop should finish quickly because task respects context")

	select {
	case <-taskExited:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Task did not exit after Runner stopped")
	}
}

func TestRunner_SubmitAndWait_Panic(t *testing.T) {
	r := NewRunner()
	r.Start(context.Background())
	defer r.Stop(context.Background())

	// 测试 Panic 场景下的同步等待
	// 如果没有修复，这里会死锁直到测试超时
	done := make(chan struct{})
	go func() {
		err := r.SubmitAndWait(context.Background(), func(ctx context.Context) {
			panic("boom")
		})
		assert.NoError(t, err) // Panic 被捕获，SubmitAndWait 应该正常返回 nil (或者根据需求设计返回错误)
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("SubmitAndWait blocked/deadlocked on panic")
	}
}
