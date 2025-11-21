# task: High-Performance In-Memory Task Runner

[![Go Report Card](https://goreportcard.com/badge/github.com/oy3o/task)](https://goreportcard.com/report/github.com/oy3o/task)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

[中文](./README.zh.md) | [English](./README.md)

`task` is a **zero-dependency**, **concurrency-controlled** in-memory task execution library for Go.

It bridges the gap between raw `go func()` (which is uncontrolled and risky) and heavy distributed queues (like RabbitMQ/Redis). It is designed to provide **Bulkheading** (resource isolation) and **Fault Tolerance** (panic recovery) for your Go services.

## Why use `task`?

*   **Protection**: Limits the number of concurrent goroutines to prevent OOM (Out of Memory).
*   **Fault Tolerance**: Automatically catches panics in tasks to keep the main process alive.
*   **Graceful Shutdown**: Ensures all running tasks are completed before the server exits (Drain pattern).
*   **Observability**: Provides real-time metrics (Active, Queued, Processed, Panics).
*   **Zero Dependency**: Built using only the Go standard library.

## Installation

```bash
go get github.com/oy3o/task
```

## Quick Start

### 1. Simple Safe Goroutine (No Pool)

If you just want to run a background job safely without managing a pool:

```go
import "github.com/oy3o/task"

func main() {
    // Replaces 'go func()'
    // Automatically recovers from panics and logs stack traces
    task.Go(context.Background(), func(ctx context.Context) {
        doWork()
    }, nil)
}
```

### 2. Worker Pool (Concurrency Limit)

For high-throughput scenarios, use a `Runner` to limit concurrency.

```go
func main() {
    // Create a runner with 10 workers and a buffer queue of 1000
    r := task.NewRunner(
        task.WithMaxWorkers(10),
        task.WithQueueSize(1000),
        task.WithErrorHandler(func(ctx context.Context, p any) {
            fmt.Printf("Task panicked: %v\n", p)
        }),
    )

    // Start workers
    r.Start(context.Background())

    // Submit tasks (Non-blocking)
    err := r.Submit(func(ctx context.Context) {
        // Do heavy work here...
        time.Sleep(time.Second)
    })

    if err == task.ErrQueueFull {
        // Handle backpressure (e.g., return 503 to client)
    }

    // Graceful Stop (Waits for running tasks to finish)
    r.Stop(context.Background())
}
```

### 3. Synchronous Wait (`SubmitAndWait`)

Useful when you need concurrency limits but need the result immediately.
**Note**: If the submitted task panics, `SubmitAndWait` will return `nil` (success) immediately after recovery, preventing deadlocks.

```go
err := r.SubmitAndWait(ctx, func(ctx context.Context) {
    // process item...
})

if err != nil {
    // Handle error (Timeout, QueueFull, or RunnerClosed)
}
```

## Observability

Monitor your worker pool health in real-time.

```go
stats := r.Stats()
fmt.Printf("Active: %d, Queued: %d, Total Processed: %d\n", 
    stats.ActiveWorkers, 
    stats.QueuedTasks, 
    stats.TotalProcessed,
)
```

## Integration with Appx

`task.Runner` implements the standard `Start/Stop` pattern, making it easy to integrate with application lifecycle managers.

```go
// In your main.go
func run() {
    runner := task.NewRunner(task.WithMaxWorkers(50))
    app := appx.New() // Your application
    
    // Dependency Injection
    app.AddService(runner) 
    
    app.Run() // Runner starts with app and stops gracefully with app
}
```

## Options

| Option | Description | Default |
| :--- | :--- | :--- |
| `WithMaxWorkers(n)` | Max number of concurrent goroutines. | 10 |
| `WithQueueSize(n)` | Max number of tasks waiting in queue. | 1000 |
| `WithErrorHandler(fn)` | Custom callback for handling panics. | Log to stdout |
