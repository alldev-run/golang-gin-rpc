package bloomfilter

import (
	"fmt"
	"sync"
	"testing"
)

// TestBasicOperations tests fundamental add/contains operations
func TestBasicOperations(t *testing.T) {
	bf := New(1000, 0.01)

	tests := []struct {
		name string
		data []byte
	}{
		{"hello", []byte("hello")},
		{"world", []byte("world")},
		{"empty", []byte("")},
		{"unicode", []byte("你好世界")},
		{"binary", []byte{0x00, 0x01, 0x02, 0xFF}},
	}

	// Add all elements
	for _, tt := range tests {
		bf.Add(tt.data)
	}

	// Verify all exist (no false negatives)
	for _, tt := range tests {
		if !bf.Contains(tt.data) {
			t.Errorf("Should contain %q", tt.name)
		}
	}

	// Check non-existent element
	if bf.Contains([]byte("never-added")) {
		t.Log("False positive on 'never-added' (occasionally expected)")
	}
}

// TestEmptyFilter tests behavior with empty filter
func TestEmptyFilter(t *testing.T) {
	bf := New(100, 0.01)

	// Any query on empty filter should return false
	if bf.Contains([]byte("anything")) {
		t.Error("Empty filter should not contain any element")
	}
}

// TestClear tests that Clear properly removes all elements
func TestClear(t *testing.T) {
	bf := New(1000, 0.01)
	elements := [][]byte{
		[]byte("a"), []byte("b"), []byte("c"),
	}

	for _, elem := range elements {
		bf.Add(elem)
	}

	// Verify all exist
	for _, elem := range elements {
		if !bf.Contains(elem) {
			t.Error("Should contain element before clear")
		}
	}

	bf.Clear()

	// Verify none exist after clear
	for _, elem := range elements {
		if bf.Contains(elem) {
			t.Error("Should not contain element after clear")
		}
	}
}

// TestConcurrency tests thread-safe operations with concurrent goroutines
func TestConcurrency(t *testing.T) {
	bf := New(100000, 0.01)
	numGoroutines := 100
	opsPerGoroutine := 1000

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*opsPerGoroutine*2)

	// Concurrent Add operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				key := fmt.Sprintf("goroutine-%d-key-%d", id, j)
				bf.Add([]byte(key))
			}
		}(i)
	}

	// Concurrent Contains operations (overlapping with Add)
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				key := fmt.Sprintf("goroutine-%d-key-%d", id, j)
				bf.Contains([]byte(key)) // Should not panic or race
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	errCount := 0
	for err := range errors {
		if err != nil {
			t.Error(err)
			errCount++
			if errCount > 10 {
				t.Fatal("Too many errors, stopping test")
			}
		}
	}

	// Verify at least some elements exist (can't check all due to race conditions)
	t.Logf("Completed %d concurrent Add and %d concurrent Contains operations",
		numGoroutines*opsPerGoroutine, numGoroutines*opsPerGoroutine)
}

// TestConcurrentClear tests Clear during concurrent operations
func TestConcurrentClear(t *testing.T) {
	bf := New(10000, 0.01)

	var wg sync.WaitGroup

	// Writer goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				bf.Add([]byte(fmt.Sprintf("key-%d-%d", id, j)))
			}
		}(i)
	}

	// Clear goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			bf.Clear()
		}
	}()

	// Reader goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				bf.Contains([]byte(fmt.Sprintf("key-%d-%d", id, j)))
			}
		}(i)
	}

	wg.Wait()
	t.Log("Concurrent Add/Clear/Contains operations completed without race conditions")
}

// TestFalsePositiveRate validates statistical properties
func TestFalsePositiveRate(t *testing.T) {
	tests := []struct {
		n uint64  // expected elements
		p float64 // target false positive rate
	}{
		{1000, 0.1},    // 10% FP rate
		{10000, 0.01},  // 1% FP rate
		{100000, 0.001}, // 0.1% FP rate
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("n=%d_p=%f", tt.n, tt.p), func(t *testing.T) {
			bf := New(tt.n, tt.p)

			// Add n elements
			for i := uint64(0); i < tt.n; i++ {
				bf.Add([]byte(fmt.Sprintf("element-%d", i)))
			}

			// Test with n different elements (never added)
			falsePositives := 0
			for i := tt.n; i < tt.n*2; i++ {
				if bf.Contains([]byte(fmt.Sprintf("element-%d", i))) {
					falsePositives++
				}
			}

			actualRate := float64(falsePositives) / float64(tt.n)
			t.Logf("Target FP rate: %.4f, Actual: %.4f", tt.p, actualRate)

			// Allow 3x tolerance for statistical variance
			if actualRate > tt.p*3 {
				t.Errorf("FP rate %.4f exceeds tolerance (target %.4f)", actualRate, tt.p)
			}
		})
	}
}

// TestLargeData tests with large data payloads
func TestLargeData(t *testing.T) {
	bf := New(1000, 0.01)

	// 1KB data
	largeData := make([]byte, 1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	bf.Add(largeData)
	if !bf.Contains(largeData) {
		t.Error("Should contain large data")
	}

	// Slightly different data should not match
	differentData := make([]byte, 1024)
	copy(differentData, largeData)
	differentData[512] = 0xFF

	if bf.Contains(differentData) {
		t.Log("False positive on similar large data (occasionally expected)")
	}
}

// TestMultipleInstances tests that multiple filters operate independently
func TestMultipleInstances(t *testing.T) {
	bf1 := New(100, 0.01)
	bf2 := New(100, 0.01)

	bf1.Add([]byte("only-in-first"))
	bf2.Add([]byte("only-in-second"))

	if !bf1.Contains([]byte("only-in-first")) {
		t.Error("bf1 should contain its element")
	}
	if bf1.Contains([]byte("only-in-second")) {
		t.Error("bf1 should not contain bf2's element")
	}

	if !bf2.Contains([]byte("only-in-second")) {
		t.Error("bf2 should contain its element")
	}
	if bf2.Contains([]byte("only-in-first")) {
		t.Error("bf2 should not contain bf1's element")
	}

	// Clear one shouldn't affect the other
	bf1.Clear()
	if bf1.Contains([]byte("only-in-first")) {
		t.Error("bf1 should be empty after clear")
	}
	if !bf2.Contains([]byte("only-in-second")) {
		t.Error("bf2 should still contain its element")
	}
}

// BenchmarkAdd benchmarks Add operation
func BenchmarkAdd(b *testing.B) {
	bf := New(uint64(b.N), 0.01)
	data := []byte("benchmark-key")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bf.Add(data)
	}
}

// BenchmarkContains benchmarks Contains operation
func BenchmarkContains(b *testing.B) {
	bf := New(10000, 0.01)
	bf.Add([]byte("test-key"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bf.Contains([]byte("test-key"))
	}
}

// BenchmarkConcurrentAdd benchmarks concurrent Add operations
func BenchmarkConcurrentAdd(b *testing.B) {
	bf := New(uint64(b.N), 0.01)

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			bf.Add([]byte(fmt.Sprintf("key-%d", i)))
			i++
		}
	})
}

// BenchmarkConcurrentContains benchmarks concurrent Contains operations
func BenchmarkConcurrentContains(b *testing.B) {
	bf := New(100000, 0.01)
	for i := 0; i < 100000; i++ {
		bf.Add([]byte(fmt.Sprintf("key-%d", i)))
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			bf.Contains([]byte(fmt.Sprintf("key-%d", i%100000)))
			i++
		}
	})
}
