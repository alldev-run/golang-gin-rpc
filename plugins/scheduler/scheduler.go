package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Task 代表一个调度任务
type Task struct {
	ID       string
	Name     string
	Interval time.Duration
	Delay    time.Duration
	Times    int // 0 表示无限
	Func     func(ctx context.Context) error
	OnError  func(error)

	// 内部字段
	mu       sync.Mutex
	nextRun  time.Time
	runCount int
	active   bool
	cancel   context.CancelFunc
}

// Scheduler 调度器
type Scheduler struct {
	tasks     sync.Map // 使用 sync.Map 减少读写锁在高并发下的竞争
	wg        sync.WaitGroup
	stopCh    chan struct{}
	running   int32 // 原子操作
	precision time.Duration
}

type Options struct {
	Precision time.Duration
}

func DefaultOptions() Options {
	return Options{Precision: time.Millisecond * 10} // 建议默认为10ms，平衡性能与精度
}

func New(opts Options) *Scheduler {
	if opts.Precision <= 0 {
		opts.Precision = time.Millisecond * 10
	}
	return &Scheduler{
		stopCh:    make(chan struct{}),
		precision: opts.Precision,
	}
}

func (s *Scheduler) Start() {
	if !atomic.CompareAndSwapInt32(&s.running, 0, 1) {
		return
	}

	s.wg.Add(1)
	go s.run()
}

func (s *Scheduler) Stop() {
	if atomic.CompareAndSwapInt32(&s.running, 1, 0) {
		close(s.stopCh)
		s.wg.Wait()

		// 停止所有正在运行的任务
		s.tasks.Range(func(key, value interface{}) bool {
			t := value.(*Task)
			t.mu.Lock()
			if t.cancel != nil {
				t.cancel()
			}
			t.mu.Unlock()
			return true
		})
	}
}

func (s *Scheduler) AddTask(task *Task) error {
	if task.ID == "" || task.Func == nil {
		return errors.New("task ID and function are required")
	}

	task.mu.Lock()
	task.active = true
	// 如果没有设置 Delay，则立即开始计算第一次执行时间
	if task.nextRun.IsZero() {
		task.nextRun = time.Now().Add(task.Delay)
	}
	task.mu.Unlock()

	if _, loaded := s.tasks.LoadOrStore(task.ID, task); loaded {
		return fmt.Errorf("task %s already exists", task.ID)
	}
	return nil
}

func (s *Scheduler) RemoveTask(id string) {
	if val, ok := s.tasks.LoadAndDelete(id); ok {
		t := val.(*Task)
		t.mu.Lock()
		t.active = false
		if t.cancel != nil {
			t.cancel()
		}
		t.mu.Unlock()
	}
}

func (s *Scheduler) run() {
	defer s.wg.Done()
	ticker := time.NewTicker(s.precision)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case now := <-ticker.C:
			s.tick(now)
		}
	}
}

func (s *Scheduler) tick(now time.Time) {
	s.tasks.Range(func(key, value interface{}) bool {
		task := value.(*Task)

		// 细粒度检查任务状态
		if !task.shouldRun(now) {
			return true
		}

		s.wg.Add(1)
		go func(t *Task) {
			defer s.wg.Done()
			s.executeTask(t)
		}(task)

		return true
	})
}

func (t *Task) shouldRun(now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.active {
		return false
	}

	if t.Times > 0 && t.runCount >= t.Times {
		t.active = false
		return false
	}

	if now.Before(t.nextRun) {
		return false
	}

	// 更新下次运行时间
	if t.Interval > 0 {
		t.nextRun = now.Add(t.Interval)
	} else {
		// 一次性任务执行后设为不活跃
		t.active = false
	}
	t.runCount++
	return true
}

func (s *Scheduler) executeTask(task *Task) {
	// 创建任务专用的 context
	ctx, cancel := context.WithCancel(context.Background())

	task.mu.Lock()
	// 如果任务在启动瞬间被停止了
	if !task.active {
		task.mu.Unlock()
		cancel()
		return
	}
	task.cancel = cancel
	task.mu.Unlock()

	defer cancel()

	if err := task.Func(ctx); err != nil && task.OnError != nil {
		task.OnError(err)
	}
}
