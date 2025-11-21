# task: 高性能内存任务执行器

[![Go Report Card](https://goreportcard.com/badge/github.com/oy3o/task)](https://goreportcard.com/report/github.com/oy3o/task)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

[中文](./README.zh.md) | [English](./README.md)

`task` 是一个**零依赖**、**并发可控**的 Go 语言内存任务执行库。

它的设计初衷是填补“原生 `go func()`（不可控、风险高）”与“重量级分布式队列（如 RabbitMQ/Redis）”之间的空白。它为您的 Go 服务提供**资源隔离 (Bulkheading)** 和 **故障隔离 (Fault Tolerance)**。

## 核心价值

*   **资源保护**: 限制最大并发协程数，防止流量突增导致 OOM（内存溢出）。
*   **故障隔离**: 自动捕获任务中的 Panic，防止单个任务崩溃导致整个进程退出。
*   **优雅停机**: 支持 Drain 模式，在服务退出前确保正在执行的任务被处理完毕，防止数据丢失。
*   **可观测性**: 提供实时指标监控（运行中、排队中、已处理、Panic 次数）。
*   **零依赖**: 仅依赖 Go 标准库实现，极致轻量。

## 安装

```bash
go get github.com/oy3o/task
```

## 快速开始

### 1. 安全启动协程 (无需池化)

如果您只是想简单地替换 `go func()` 以获得 Panic 保护：

```go
import "github.com/oy3o/task"

func main() {
    // 替代 'go func()'
    // 自动恢复 Panic 并打印堆栈信息
    task.Go(context.Background(), func(ctx context.Context) {
        doWork()
    }, nil)
}
```

### 2. Worker Pool (并发限制)

在高吞吐场景下，使用 `Runner` 来限制并发数。

```go
func main() {
    // 创建 Runner：10 个并发 Worker，队列缓冲 1000
    r := task.NewRunner(
        task.WithMaxWorkers(10),
        task.WithQueueSize(1000),
        task.WithErrorHandler(func(ctx context.Context, p any) {
            fmt.Printf("任务发生 Panic: %v\n", p)
        }),
    )

    // 启动 Workers
    r.Start(context.Background())

    // 提交任务 (非阻塞)
    err := r.Submit(func(ctx context.Context) {
        // 执行耗时逻辑...
        time.Sleep(time.Second)
    })

    if err == task.ErrQueueFull {
        // 处理背压 (例如：向客户端返回 503)
    }

    // 优雅停止 (等待所有正在运行的任务完成)
    r.Stop(context.Background())
}
```

### 3. 同步等待 (`SubmitAndWait`)

适用于需要并发限制，但当前请求必须等待任务完成的场景（例如：HTTP 请求中的批量处理）。
**注意**：如果提交的任务发生 panic，`SubmitAndWait` 将在恢复后立即返回 `nil`（成功），从而防止死锁。

```go
err := r.SubmitAndWait(ctx, func(ctx context.Context) {
    // 处理单个条目...
})

if err != nil {
    // 处理错误 (超时、队列已满 或 Runner 已关闭)
}
```

## 可观测性 (Observability)

您可以实时监控 Runner 的健康状态，用于 HPA（自动扩缩容）或健康检查。

```go
stats := r.Stats()
fmt.Printf("运行中: %d, 排队中: %d, 总处理: %d, 拒绝数: %d\n", 
    stats.ActiveWorkers, 
    stats.QueuedTasks, 
    stats.TotalProcessed,
    stats.TotalRefused,
)
```

## 与 Appx 集成

`task.Runner` 实现了标准的 `Start/Stop` 生命周期接口，可以轻松挂载到应用程序管理器中。

```go
// 在 main.go 中
func run() {
    runner := task.NewRunner(task.WithMaxWorkers(50))
    app := appx.New() // 您的应用
    
    // 依赖注入：将 Runner 托管给 app
    app.AddService(runner) 
    
    // Runner 会随 app 启动，并随 app 优雅退出
    app.Run() 
}
```

## 配置选项

| 选项 | 说明 | 默认值 |
| :--- | :--- | :--- |
| `WithMaxWorkers(n)` | 最大并发 Worker 数 (并发协程数) | 10 |
| `WithQueueSize(n)` | 等待队列的最大容量 | 1000 |
| `WithErrorHandler(fn)` | Panic 发生时的自定义回调函数 | 打印到标准输出 |
