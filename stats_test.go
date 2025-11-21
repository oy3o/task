package task

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRunner_Stats(t *testing.T) {
	r := NewRunner(WithMaxWorkers(1), WithQueueSize(10))
	r.Start(context.Background())
	defer r.Stop(context.Background())

	// 初始状态
	s := r.Stats()
	assert.Equal(t, 0, int(s.ActiveWorkers))
	assert.Equal(t, 0, s.QueuedTasks)
	assert.Equal(t, uint64(0), s.TotalProcessed)

	// 阻塞控制
	blocker := make(chan struct{})
	taskStarted := make(chan struct{})

	// 1. 提交一个阻塞任务
	r.Submit(func(ctx context.Context) {
		close(taskStarted)
		<-blocker
	})

	<-taskStarted // 等待任务开始运行

	// 2. 提交一个排队任务
	r.Submit(func(ctx context.Context) {})

	// 检查中间状态
	s = r.Stats()
	assert.Equal(t, int64(1), s.ActiveWorkers, "Active worker should be 1")
	assert.Equal(t, 1, s.QueuedTasks, "Queue should have 1 task")

	// 释放
	close(blocker)

	// 等待执行完成 (简单 sleep，生产环境可以用 waitgroup)
	time.Sleep(50 * time.Millisecond)

	// 检查最终状态
	s = r.Stats()
	assert.Equal(t, int64(0), s.ActiveWorkers)
	assert.Equal(t, 0, s.QueuedTasks)
	assert.Equal(t, uint64(2), s.TotalProcessed)
}

func TestRunner_Stats_Panic(t *testing.T) {
	r := NewRunner(WithMaxWorkers(1))
	r.Start(context.Background())
	defer r.Stop(context.Background())

	// 提交一个 Panic 任务
	r.Submit(func(ctx context.Context) {
		panic("oops")
	})

	time.Sleep(50 * time.Millisecond)

	s := r.Stats()
	assert.Equal(t, uint64(1), s.TotalPanics)
	assert.Equal(t, uint64(1), s.TotalProcessed) // Panic 也算处理完成
}
