// Package scheduler provides millisecond-precision task scheduling
package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Task represents a scheduled task
type Task struct {
	ID       string
	Name     string
	Interval time.Duration
	Delay    time.Duration
	Times    int // 0 means unlimited
	Cron     string
	Func     func(ctx context.Context) error
	OnError  func(error)

	// Internal fields
	nextRun  time.Time
	runCount int
	active   bool
	cancel   context.CancelFunc
}

// Scheduler manages scheduled tasks
type Scheduler struct {
	tasks    map[string]*Task
	mu       sync.RWMutex
	wg       sync.WaitGroup
	stopCh   chan struct{}
	running  bool
	ticker   *time.Ticker
	precision time.Duration
}

// Options scheduler options
type Options struct {
	Precision time.Duration // default 1ms
}

// DefaultOptions returns default options
func DefaultOptions() Options {
	return Options{
		Precision: time.Millisecond,
	}
}

// New creates a new scheduler
func New(opts Options) *Scheduler {
	if opts.Precision <= 0 {
		opts.Precision = time.Millisecond
	}

	return &Scheduler{
		tasks:     make(map[string]*Task),
		stopCh:    make(chan struct{}),
		precision: opts.Precision,
	}
}

// Start starts the scheduler
func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return
	}

	s.running = true
	s.ticker = time.NewTicker(s.precision)

	s.wg.Add(1)
	go s.run()
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.ticker.Stop()
	close(s.stopCh)
	s.mu.Unlock()

	s.wg.Wait()
}

// AddTask adds a new task
func (s *Scheduler) AddTask(task *Task) error {
	if task.ID == "" {
		return fmt.Errorf("task ID is required")
	}
	if task.Func == nil {
		return fmt.Errorf("task function is required")
	}
	if task.Interval <= 0 && task.Cron == "" {
		return fmt.Errorf("task interval or cron expression is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[task.ID]; exists {
		return fmt.Errorf("task %s already exists", task.ID)
	}

	task.active = true
	task.nextRun = time.Now().Add(task.Delay)
	s.tasks[task.ID] = task

	return nil
}

// RemoveTask removes a task
func (s *Scheduler) RemoveTask(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task, exists := s.tasks[id]; exists {
		task.active = false
		if task.cancel != nil {
			task.cancel()
		}
		delete(s.tasks, id)
	}
}

// PauseTask pauses a task
func (s *Scheduler) PauseTask(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task, exists := s.tasks[id]; exists {
		task.active = false
	}
}

// ResumeTask resumes a paused task
func (s *Scheduler) ResumeTask(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task, exists := s.tasks[id]; exists {
		task.active = true
		task.nextRun = time.Now().Add(task.Interval)
	}
}

// GetTask returns a task by ID
func (s *Scheduler) GetTask(id string) (*Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[id]
	return task, exists
}

// ListTasks returns all tasks
func (s *Scheduler) ListTasks() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// run is the main scheduling loop
func (s *Scheduler) run() {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopCh:
			return
		case now := <-s.ticker.C:
			s.checkAndRun(now)
		}
	}
}

// checkAndRun checks tasks and runs due ones
func (s *Scheduler) checkAndRun(now time.Time) {
	s.mu.Lock()
	tasksToRun := make([]*Task, 0)

	for _, task := range s.tasks {
		if !task.active {
			continue
		}

		if task.Times > 0 && task.runCount >= task.Times {
			task.active = false
			continue
		}

		if now.After(task.nextRun) || now.Equal(task.nextRun) {
			tasksToRun = append(tasksToRun, task)
			task.nextRun = now.Add(task.Interval)
			task.runCount++
		}
	}
	s.mu.Unlock()

	// Run tasks concurrently
	for _, task := range tasksToRun {
		s.wg.Add(1)
		go func(t *Task) {
			defer s.wg.Done()
			s.executeTask(t)
		}(task)
	}
}

// executeTask executes a single task
func (s *Scheduler) executeTask(task *Task) {
	ctx, cancel := context.WithTimeout(context.Background(), task.Interval)
	defer cancel()

	task.cancel = cancel

	if err := task.Func(ctx); err != nil && task.OnError != nil {
		task.OnError(err)
	}
}

// Once creates a one-time task
func Once(id string, delay time.Duration, fn func(ctx context.Context) error) *Task {
	return &Task{
		ID:       id,
		Name:     id,
		Delay:    delay,
		Interval: time.Hour * 24 * 365 * 100, // 100 years, effectively one-time
		Times:    1,
		Func:     fn,
	}
}

// Every creates a periodic task
func Every(id string, interval time.Duration, fn func(ctx context.Context) error) *Task {
	return &Task{
		ID:       id,
		Name:     id,
		Interval: interval,
		Func:     fn,
	}
}

// Times creates a task that runs specific times
func Times(id string, interval time.Duration, times int, fn func(ctx context.Context) error) *Task {
	return &Task{
		ID:       id,
		Name:     id,
		Interval: interval,
		Times:    times,
		Func:     fn,
	}
}

// Delayed creates a delayed periodic task
func Delayed(id string, delay, interval time.Duration, fn func(ctx context.Context) error) *Task {
	return &Task{
		ID:       id,
		Name:     id,
		Delay:    delay,
		Interval: interval,
		Func:     fn,
	}
}

// ScheduleFunc schedules a function with milliseconds precision
func (s *Scheduler) ScheduleFunc(id string, delayMs int64, fn func()) {
	task := Once(id, time.Duration(delayMs)*time.Millisecond, func(ctx context.Context) error {
		fn()
		return nil
	})
	s.AddTask(task)
}

// ScheduleRepeat schedules a repeating function with milliseconds precision
func (s *Scheduler) ScheduleRepeat(id string, intervalMs int64, fn func()) {
	task := Every(id, time.Duration(intervalMs)*time.Millisecond, func(ctx context.Context) error {
		fn()
		return nil
	})
	s.AddTask(task)
}
