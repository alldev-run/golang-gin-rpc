package aoi

import (
	"sync"
	"testing"
)

func TestEnterAndAround(t *testing.T) {
	m := NewManager(100)
	m.Enter(1, 10, 10)
	m.Enter(2, 20, 20)   // 同格
	m.Enter(3, 150, 150) // 相邻格
	m.Enter(4, 350, 350) // 远处,不应出现

	got := map[int64]bool{}
	for _, e := range m.Around(1) {
		got[e.ID] = true
	}
	if !got[2] || !got[3] {
		t.Fatalf("expected neighbors {2,3}, got %v", got)
	}
	if got[4] {
		t.Fatal("far entity should not be visible")
	}
}

func TestAroundExcludesSelf(t *testing.T) {
	m := NewManager(100)
	m.Enter(1, 5, 5)
	m.Enter(2, 6, 6)
	for _, e := range m.Around(1) {
		if e.ID == 1 {
			t.Fatal("Around should exclude self")
		}
	}
}

func TestMoveAcrossCells(t *testing.T) {
	m := NewManager(100)
	m.Enter(1, 10, 10)
	m.Enter(2, 500, 500)
	if len(m.Around(2)) != 0 {
		t.Fatal("should be far apart initially")
	}
	m.Move(1, 505, 505) // 移入 2 的邻格
	if len(m.Around(2)) != 1 {
		t.Fatalf("expected 1 neighbor after move, got %d", len(m.Around(2)))
	}
	if _, ok := m.Get(1); !ok {
		t.Fatal("entity should exist")
	}
}

func TestNegativeCoordinates(t *testing.T) {
	m := NewManager(100)
	m.Enter(1, -10, -10)
	m.Enter(2, -20, -20) // 同格(-1,-1)
	m.Enter(3, 250, 250) // (2,2) 格,超出 3x3
	if len(m.Around(1)) != 1 {
		t.Fatalf("expected 1 neighbor, got %d", len(m.Around(1)))
	}
	if _, ok := m.Get(1); !ok {
		t.Fatal("negative-coordinate entity should exist")
	}
}

func TestLeave(t *testing.T) {
	m := NewManager(100)
	m.Enter(1, 10, 10)
	m.Enter(2, 15, 15)
	if !m.Leave(2) {
		t.Fatal("Leave should return true for existing entity")
	}
	if m.Leave(2) {
		t.Fatal("double Leave should return false")
	}
	if n := len(m.Around(1)); n != 0 {
		t.Fatalf("expected 0 neighbors, got %d", n)
	}
	if m.Count() != 1 {
		t.Fatalf("count = %d", m.Count())
	}
}

func TestAroundPoint(t *testing.T) {
	m := NewManager(100)
	m.Enter(1, 10, 10)
	m.Enter(2, 800, 800)
	if got := len(m.AroundPoint(50, 50)); got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
	if got := len(m.AroundPoint(800, 800)); got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := NewManager(100)
	var wg sync.WaitGroup
	for id := int64(1); id <= 50; id++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			m.Enter(id, float64(id)*3, float64(id)*3)
			for step := 0; step < 20; step++ {
				m.Move(id, float64(id)*3+float64(step), float64(id)*3+float64(step))
				_ = m.Around(id)
			}
			m.Leave(id)
		}(id)
	}
	wg.Wait()
	if m.Count() != 0 {
		t.Fatalf("count = %d, want 0", m.Count())
	}
}
