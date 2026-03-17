# Scheduler

毫秒级精度的定时任务调度器，支持周期性任务、延迟任务和指定执行次数。

## 功能特性

- **毫秒级精度**：默认 1ms 精度，支持微秒级配置
- **多种任务类型**：一次性、周期性、延迟执行、指定次数
- **任务管理**：支持暂停、恢复、删除任务
- **错误处理**：支持错误回调
- **上下文支持**：支持超时和取消
- **并发安全**：线程安全的任务操作

## 安装

```go
import "alldev-gin-rpc/pkg/scheduler"
```

## 快速开始

### 创建调度器

```go
// 默认配置（1ms精度）
s := scheduler.New(scheduler.DefaultOptions())

// 自定义精度（10ms）
s := scheduler.New(scheduler.Options{Precision: 10 * time.Millisecond})

s.Start()
defer s.Stop()
```

### 创建任务

```go
// 一次性任务（延迟100ms执行）
task := scheduler.Once("cleanup", 100*time.Millisecond, func(ctx context.Context) error {
    return cleanupTempFiles()
})
s.AddTask(task)

// 周期性任务（每500ms执行一次）
task := scheduler.Every("heartbeat", 500*time.Millisecond, func(ctx context.Context) error {
    return sendHeartbeat()
})
s.AddTask(task)

// 指定执行3次
task := scheduler.Times("retry", time.Second, 3, func(ctx context.Context) error {
    return tryConnect()
})
s.AddTask(task)

// 延迟后周期性执行
task := scheduler.Delayed("warmup", 5*time.Second, time.Minute, func(ctx context.Context) error {
    return warmupCache()
})
s.AddTask(task)
```

### 便捷方法

```go
// 毫秒级延迟任务
s.ScheduleFunc("task1", 100, func() {
    fmt.Println("100ms后执行")
})

// 毫秒级周期性任务
s.ScheduleRepeat("task2", 500, func() {
    fmt.Println("每500ms执行")
})
```

## 任务管理

```go
// 暂停任务
s.PauseTask("heartbeat")

// 恢复任务
s.ResumeTask("heartbeat")

// 删除任务
s.RemoveTask("cleanup")

// 查询任务
task, exists := s.GetTask("heartbeat")

// 列出所有任务
tasks := s.ListTasks()
```

## 错误处理

```go
task := scheduler.Every("job", time.Minute, func(ctx context.Context) error {
    return doWork()
})

task.OnError = func(err error) {
    log.Printf("任务执行失败: %v", err)
    alert.Send("任务异常", err.Error())
}

s.AddTask(task)
```

## 高级用法

### 带超时的任务

```go
task := &scheduler.Task{
    ID:       "slow-job",
    Interval: 30 * time.Second,
    Func: func(ctx context.Context) error {
        // 任务超时时间 = Interval
        return longRunningWork(ctx)
    },
    OnError: func(err error) {
        if err == context.DeadlineExceeded {
            log.Println("任务执行超时")
        }
    },
}
s.AddTask(task)
```

### 动态修改任务

```go
// 获取任务并修改
task, _ := s.GetTask("dynamic")
task.Interval = 2 * time.Minute  // 修改执行间隔
```

## 配置选项

| 选项 | 说明 | 默认值 |
|------|------|--------|
| Precision | 调度精度 | 1ms |

## 完整示例

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "alldev-gin-rpc/pkg/scheduler"
)

func main() {
    // 创建调度器
    s := scheduler.New(scheduler.DefaultOptions())
    s.Start()
    defer s.Stop()
    
    // 启动任务
    s.ScheduleRepeat("counter", 1000, func() {
        fmt.Println("Tick every second")
    })
    
    // 10秒后执行清理
    s.ScheduleFunc("cleanup", 10000, func() {
        fmt.Println("Cleanup after 10s")
    })
    
    // 运行20秒
    time.Sleep(20 * time.Second)
}
```

## 性能

- 支持数千个并发任务
- 内存占用低（每个任务约 200 bytes）
- 调度延迟 < 1ms

## 注意事项

1. 任务函数应该尽快返回，长时间任务应使用 goroutine
2. 任务执行时间不应超过 Interval，否则会被取消
3. 调度器停止时会等待所有进行中的任务完成
