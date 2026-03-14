package task

import (
	"context"
	"sync"
	"sync/atomic"
)

// 复用信号 Channel 的 Pool
var doneChanPool = sync.Pool{
	New: func() any {
		// 必须是有缓冲的，防止 Worker 在 Caller 超时后阻塞
		return make(chan struct{}, 1)
	},
}

// TaskFunc 定义了要执行的任务函数签名
type TaskFunc func(ctx context.Context)

// Runner 是一个并发限制的任务执行器。
type Runner struct {
	cfg       *config
	taskChan  chan TaskFunc
	wg        sync.WaitGroup
	running   atomic.Bool
	startOnce sync.Once
	stopOnce  sync.Once

	// 生命周期管理
	// ctx 用于通知所有 Worker 和正在运行的任务：Runner 正在停止
	ctx    context.Context
	cancel context.CancelFunc

	// mu 保护 taskChan 的关闭操作，防止 "send on closed channel" panic
	mu sync.RWMutex

	// --- 内部指标计数器 ---
	activeWorkers  atomic.Int64
	statsProcessed atomic.Uint64
	statsPanics    atomic.Uint64
	statsRefused   atomic.Uint64
}

func NewRunner(opts ...Option) *Runner {
	c := DefaultConfig()
	for _, opt := range opts {
		opt(c)
	}

	return &Runner{
		cfg:      c,
		taskChan: make(chan TaskFunc, c.queueSize),
	}
}

func (r *Runner) Start(ctx context.Context) error {
	r.startOnce.Do(func() {
		// 绑定生命周期 Context 到 caller 传入的 ctx
		r.ctx, r.cancel = context.WithCancel(ctx)
		r.running.Store(true)
		for i := 0; i < r.cfg.maxWorkers; i++ {
			r.wg.Add(1)
			go r.worker()
		}
	})
	return nil
}

func (r *Runner) Stop(ctx context.Context) error {
	var err error
	r.stopOnce.Do(func() {
		// 1. 标记停止状态
		r.running.Store(false)

		// 2. 取消 Runner 的 Context，通知正在运行的任务（如果任务监听了 ctx.Done()）
		r.cancel()

		// 3. 加写锁关闭 channel
		// 确保此时没有 Submit 操作持有读锁正在写入
		r.mu.Lock()
		close(r.taskChan)
		r.mu.Unlock()

		// 4. 等待所有 Worker 处理完积压任务
		done := make(chan struct{})
		go func() {
			r.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			return
		case <-ctx.Done():
			err = ctx.Err()
		}
	})
	return err
}

// Submit 提交异步任务
func (r *Runner) Submit(task TaskFunc) error {
	// 1. 快速检查 (性能优化，无锁)
	if !r.running.Load() {
		return ErrRunnerClosed
	}

	// 2. 加读锁保护 channel 写入
	// 防止在 select 写入时，Stop 方法并发关闭 channel
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 3. 双重检查 (防止在获取锁的过程中状态改变)
	if !r.running.Load() {
		return ErrRunnerClosed
	}

	select {
	case r.taskChan <- task:
		return nil
	default:
		r.statsRefused.Add(1) // 记录拒绝数
		return ErrQueueFull
	}
}

// SubmitAndWait 提交任务并等待完成 (同步模式)
// 适用于 HTTP 请求中需要并发处理但必须等待结果的场景。
func (r *Runner) SubmitAndWait(ctx context.Context, task TaskFunc) error {
	done := doneChanPool.Get().(chan struct{})

	// 防御性编程：清空脏信号
	select {
	case <-done:
	default:
	}

	err := r.Submit(func(runnerCtx context.Context) {
		defer func() {
			done <- struct{}{}
		}()
		// 核心修复: 注入 Caller 的 Context。
		// 任务的生命周期现在与请求方绑定，而不再是全局 Runner 的上下文。
		task(ctx)
	})

	if err != nil {
		doneChanPool.Put(done)
		return err
	}

	select {
	case <-done:
		doneChanPool.Put(done)
		return nil
	case <-ctx.Done():
		// 任务超时或被取消
		// 注意：此时 Worker 可能还在运行，或者排队中。它最终会往 done 写入数据。
		// 如果我们现在把 done 放回 Pool，下一个请求拿到它时，可能会读到上一个 Worker 写入的“脏信号”。
		// 策略：直接丢弃该 Channel，不放回 Pool。让 GC 回收它。
		// 这在超时场景下退化为原始性能，但在正常场景下获得了 Pool 的性能优势。
		return ctx.Err()
	}
}

func (r *Runner) worker() {
	defer r.wg.Done()
	ctx := r.ctx

	for task := range r.taskChan {
		r.activeWorkers.Add(1)

		// 核心修复: 通过 Safely 的返回值精确判断状态机走向
		panicked := Safely(ctx, task, r.cfg.errHandler)

		r.statsProcessed.Add(1)
		if panicked {
			r.statsPanics.Add(1)
		}

		r.activeWorkers.Add(-1)
	}
}
