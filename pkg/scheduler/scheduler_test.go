package scheduler

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	s := New(DefaultOptions())
	if s == nil {
		t.Fatal("expected scheduler to be created")
	}
	if s.precision != time.Millisecond {
		t.Errorf("expected default precision 1ms, got %v", s.precision)
	}
}

func TestNewWithCustomPrecision(t *testing.T) {
	opts := Options{Precision: 10 * time.Millisecond}
	s := New(opts)
	if s.precision != 10*time.Millisecond {
		t.Errorf("expected precision 10ms, got %v", s.precision)
	}
}

func TestScheduler_StartStop(t *testing.T) {
	s := New(DefaultOptions())
	s.Start()
	if !s.running {
		t.Error("expected scheduler to be running")
	}
	s.Stop()
	if s.running {
		t.Error("expected scheduler to be stopped")
	}
}

func TestScheduler_AddTask(t *testing.T) {
	s := New(DefaultOptions())

	// Valid task
	task := &Task{
		ID:       "test-1",
		Interval: time.Second,
		Func: func(ctx context.Context) error {
			return nil
		},
	}
	if err := s.AddTask(task); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Duplicate ID
	if err := s.AddTask(task); err == nil {
		t.Error("expected error for duplicate ID")
	}

	// Empty ID
	task2 := &Task{
		ID:       "",
		Interval: time.Second,
		Func:     func(ctx context.Context) error { return nil },
	}
	if err := s.AddTask(task2); err == nil {
		t.Error("expected error for empty ID")
	}

	// Nil function
	task3 := &Task{
		ID:       "test-3",
		Interval: time.Second,
	}
	if err := s.AddTask(task3); err == nil {
		t.Error("expected error for nil function")
	}

	// No interval or cron
	task4 := &Task{
		ID:   "test-4",
		Func: func(ctx context.Context) error { return nil },
	}
	if err := s.AddTask(task4); err == nil {
		t.Error("expected error for no interval")
	}
}

func TestScheduler_RemoveTask(t *testing.T) {
	s := New(DefaultOptions())

	task := &Task{
		ID:       "test-remove",
		Interval: time.Second,
		Func:     func(ctx context.Context) error { return nil },
	}
	s.AddTask(task)

	s.RemoveTask("test-remove")

	_, exists := s.GetTask("test-remove")
	if exists {
		t.Error("expected task to be removed")
	}
}

func TestScheduler_PauseResumeTask(t *testing.T) {
	s := New(DefaultOptions())

	task := &Task{
		ID:       "test-pause",
		Interval: time.Second,
		Func:     func(ctx context.Context) error { return nil },
	}
	s.AddTask(task)

	s.PauseTask("test-pause")
	task, _ = s.GetTask("test-pause")
	if task.active {
		t.Error("expected task to be paused")
	}

	s.ResumeTask("test-pause")
	task, _ = s.GetTask("test-pause")
	if !task.active {
		t.Error("expected task to be resumed")
	}
}

func TestScheduler_ListTasks(t *testing.T) {
	s := New(DefaultOptions())

	s.AddTask(&Task{
		ID:       "task-1",
		Interval: time.Second,
		Func:     func(ctx context.Context) error { return nil },
	})
	s.AddTask(&Task{
		ID:       "task-2",
		Interval: time.Second,
		Func:     func(ctx context.Context) error { return nil },
	})

	tasks := s.ListTasks()
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestOnce(t *testing.T) {
	task := Once("once-task", 100*time.Millisecond, func(ctx context.Context) error {
		return nil
	})

	if task.ID != "once-task" {
		t.Errorf("expected ID 'once-task', got '%s'", task.ID)
	}
	if task.Times != 1 {
		t.Errorf("expected Times=1, got %d", task.Times)
	}
	if task.Delay != 100*time.Millisecond {
		t.Errorf("expected Delay=100ms, got %v", task.Delay)
	}
}

func TestEvery(t *testing.T) {
	task := Every("every-task", 500*time.Millisecond, func(ctx context.Context) error {
		return nil
	})

	if task.ID != "every-task" {
		t.Errorf("expected ID 'every-task', got '%s'", task.ID)
	}
	if task.Interval != 500*time.Millisecond {
		t.Errorf("expected Interval=500ms, got %v", task.Interval)
	}
	if task.Times != 0 {
		t.Errorf("expected Times=0 (unlimited), got %d", task.Times)
	}
}

func TestTimes(t *testing.T) {
	task := Times("times-task", 100*time.Millisecond, 5, func(ctx context.Context) error {
		return nil
	})

	if task.Times != 5 {
		t.Errorf("expected Times=5, got %d", task.Times)
	}
}

func TestDelayed(t *testing.T) {
	task := Delayed("delayed-task", 200*time.Millisecond, 100*time.Millisecond, func(ctx context.Context) error {
		return nil
	})

	if task.Delay != 200*time.Millisecond {
		t.Errorf("expected Delay=200ms, got %v", task.Delay)
	}
	if task.Interval != 100*time.Millisecond {
		t.Errorf("expected Interval=100ms, got %v", task.Interval)
	}
}

func TestScheduleFunc(t *testing.T) {
	s := New(DefaultOptions())
	s.Start()
	defer s.Stop()

	var executed int32
	s.ScheduleFunc("schedule-func", 50, func() {
		atomic.AddInt32(&executed, 1)
	})

	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&executed) != 1 {
		t.Errorf("expected task to execute once, got %d", atomic.LoadInt32(&executed))
	}
}

func TestScheduleRepeat(t *testing.T) {
	s := New(DefaultOptions())
	s.Start()
	defer s.Stop()

	var executed int32
	s.ScheduleRepeat("repeat-func", 50, func() {
		atomic.AddInt32(&executed, 1)
	})

	time.Sleep(200 * time.Millisecond)

	count := atomic.LoadInt32(&executed)
	if count < 2 {
		t.Errorf("expected at least 2 executions, got %d", count)
	}
}

func TestTaskExecution(t *testing.T) {
	s := New(DefaultOptions())
	s.Start()
	defer s.Stop()

	var executed int32
	task := &Task{
		ID:       "exec-test",
		Interval: 50 * time.Millisecond,
		Times:    3,
		Func: func(ctx context.Context) error {
			atomic.AddInt32(&executed, 1)
			return nil
		},
	}

	s.AddTask(task)
	time.Sleep(300 * time.Millisecond)

	count := atomic.LoadInt32(&executed)
	if count != 3 {
		t.Errorf("expected 3 executions, got %d", count)
	}
}

func TestTaskErrorCallback(t *testing.T) {
	s := New(DefaultOptions())
	s.Start()
	defer s.Stop()

	var errorCalled int32
	task := &Task{
		ID:       "error-test",
		Interval: 50 * time.Millisecond,
		Times:    1,
		Func: func(ctx context.Context) error {
			return context.DeadlineExceeded
		},
		OnError: func(err error) {
			if err == context.DeadlineExceeded {
				atomic.AddInt32(&errorCalled, 1)
			}
		},
	}

	s.AddTask(task)
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&errorCalled) != 1 {
		t.Errorf("expected error callback to be called once, got %d", atomic.LoadInt32(&errorCalled))
	}
}

func TestTaskWithDelay(t *testing.T) {
	s := New(DefaultOptions())
	s.Start()
	defer s.Stop()

	var executed int32
	task := &Task{
		ID:       "delay-test",
		Delay:    150 * time.Millisecond,
		Interval: 50 * time.Millisecond,
		Times:    1,
		Func: func(ctx context.Context) error {
			atomic.AddInt32(&executed, 1)
			return nil
		},
	}

	s.AddTask(task)

	// Check not executed immediately
	time.Sleep(50 * time.Millisecond)
	if atomic.LoadInt32(&executed) != 0 {
		t.Error("expected task not to execute before delay")
	}

	// Check executed after delay
	time.Sleep(150 * time.Millisecond)
	if atomic.LoadInt32(&executed) != 1 {
		t.Errorf("expected task to execute after delay, got %d", atomic.LoadInt32(&executed))
	}
}

func TestMultipleTasks(t *testing.T) {
	s := New(DefaultOptions())
	s.Start()
	defer s.Stop()

	var count1, count2 int32

	s.AddTask(&Task{
		ID:       "multi-1",
		Interval: 50 * time.Millisecond,
		Times:    2,
		Func: func(ctx context.Context) error {
			atomic.AddInt32(&count1, 1)
			return nil
		},
	})

	s.AddTask(&Task{
		ID:       "multi-2",
		Interval: 50 * time.Millisecond,
		Times:    2,
		Func: func(ctx context.Context) error {
			atomic.AddInt32(&count2, 1)
			return nil
		},
	})

	time.Sleep(200 * time.Millisecond)

	if atomic.LoadInt32(&count1) != 2 {
		t.Errorf("expected task1 to execute 2 times, got %d", atomic.LoadInt32(&count1))
	}
	if atomic.LoadInt32(&count2) != 2 {
		t.Errorf("expected task2 to execute 2 times, got %d", atomic.LoadInt32(&count2))
	}
}

func BenchmarkScheduler(b *testing.B) {
	s := New(Options{Precision: 10 * time.Millisecond})
	s.Start()
	defer s.Stop()

	var counter int32
	for i := 0; i < b.N; i++ {
		s.AddTask(&Task{
			ID:       fmt.Sprintf("bench-%d", i),
			Interval: time.Hour,
			Func: func(ctx context.Context) error {
				atomic.AddInt32(&counter, 1)
				return nil
			},
		})
	}
}
