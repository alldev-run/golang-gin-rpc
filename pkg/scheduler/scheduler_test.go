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
	if s.precision != 10*time.Millisecond {
		t.Errorf("expected default precision 10ms, got %v", s.precision)
	}
}

func TestNewWithCustomPrecision(t *testing.T) {
	opts := Options{Precision: 50 * time.Millisecond}
	s := New(opts)
	if s.precision != 50*time.Millisecond {
		t.Errorf("expected precision 50ms, got %v", s.precision)
	}
}

func TestScheduler_StartStop(t *testing.T) {
	s := New(DefaultOptions())
	s.Start()
	if atomic.LoadInt32(&s.running) == 0 {
		t.Error("expected scheduler to be running")
	}
	s.Stop()
	if atomic.LoadInt32(&s.running) != 0 {
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

	// Verify removal by trying to add again (should not error if removed)
	task2 := &Task{
		ID:       "test-remove",
		Interval: time.Second,
		Func:     func(ctx context.Context) error { return nil },
	}
	if err := s.AddTask(task2); err != nil {
		t.Errorf("expected to add task after removal, got error: %v", err)
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

func TestTaskOnce(t *testing.T) {
	s := New(DefaultOptions())
	s.Start()
	defer s.Stop()

	var executed int32
	task := &Task{
		ID:       "once-test",
		Delay:    50 * time.Millisecond,
		Interval: 10 * time.Millisecond, // Small interval for single execution
		Times:    1,
		Func: func(ctx context.Context) error {
			atomic.AddInt32(&executed, 1)
			return nil
		},
	}

	s.AddTask(task)
	time.Sleep(200 * time.Millisecond)

	if atomic.LoadInt32(&executed) != 1 {
		t.Errorf("expected task to execute exactly once, got %d", atomic.LoadInt32(&executed))
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
