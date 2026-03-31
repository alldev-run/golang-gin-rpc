package utils

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
)

// TestConcurrentRandomString tests the thread safety of RandomString function
func TestConcurrentRandomString(t *testing.T) {
	const numGoroutines = 100
	const numIterations = 10
	
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	
	errors := make(chan error, numGoroutines*numIterations)
	
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				str, err := RandomString(16)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d, iteration %d: %v", id, j, err)
					return
				}
				if len(str) != 16 {
					errors <- fmt.Errorf("goroutine %d, iteration %d: expected length 16, got %d", id, j, len(str))
					return
				}
			}
		}(i)
	}
	
	wg.Wait()
	close(errors)
	
	for err := range errors {
		t.Error(err)
	}
}

// TestConcurrentRandomHex tests the thread safety of RandomHex function
func TestConcurrentRandomHex(t *testing.T) {
	const numGoroutines = 50
	const numIterations = 5
	
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	
	errors := make(chan error, numGoroutines*numIterations)
	
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				hex, err := RandomHex(8)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d, iteration %d: %v", id, j, err)
					return
				}
				if len(hex) != 16 { // 8 bytes = 16 hex chars
					errors <- fmt.Errorf("goroutine %d, iteration %d: expected length 16, got %d", id, j, len(hex))
					return
				}
			}
		}(i)
	}
	
	wg.Wait()
	close(errors)
	
	for err := range errors {
		t.Error(err)
	}
}

// TestConcurrentStringOperations tests thread safety of string operations
func TestConcurrentStringOperations(t *testing.T) {
	const numGoroutines = 50
	
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	
	testData := [][]string{
		{"a", "b", "a", "c", "b"},
		{"  hello  ", " world ", "  test  "},
		{"a", "", "b", "", "c"},
		{"single"},
		{},
	}
	
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			
			for _, data := range testData {
				// Test StringRemoveDuplicates
				deduped := StringRemoveDuplicates(data)
				if len(deduped) > len(data) {
					t.Errorf("goroutine %d: StringRemoveDuplicates returned more elements than input", id)
				}
				
				// Test StringTrimSlice
				trimmed := StringTrimSlice(data)
				if len(trimmed) != len(data) {
					t.Errorf("goroutine %d: StringTrimSlice changed slice length", id)
				}
				
				// Test StringFilterEmpty
				filtered := StringFilterEmpty(data)
				for _, s := range filtered {
					if s == "" {
						t.Errorf("goroutine %d: StringFilterEmpty returned empty string", id)
					}
				}
				
				// Test StringJoinNonEmpty
				joined := StringJoinNonEmpty(data, ",")
				if len(data) > 0 && len(joined) == 0 {
					t.Errorf("goroutine %d: StringJoinNonEmpty returned empty for non-empty input", id)
				}
			}
		}(i)
	}
	
	wg.Wait()
}

// TestConcurrentJSONOperations tests thread safety of JSON operations
func TestConcurrentJSONOperations(t *testing.T) {
	const numGoroutines = 30
	
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	
	testData := map[string]interface{}{
		"name":  "test",
		"value": 123,
		"items": []string{"a", "b", "c"},
	}
	
	errors := make(chan error, numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			
			// Test ToJSON
			jsonStr, err := ToJSON(testData)
			if err != nil {
				errors <- fmt.Errorf("goroutine %d: ToJSON failed: %v", id, err)
				return
			}
			
			// Test FromJSON
			var result map[string]interface{}
			err = FromJSON(jsonStr, &result)
			if err != nil {
				errors <- fmt.Errorf("goroutine %d: FromJSON failed: %v", id, err)
				return
			}
			
			// Test SafeFromJSON
			var safeResult map[string]interface{}
			err = SafeFromJSON(jsonStr, &safeResult)
			if err != nil {
				errors <- fmt.Errorf("goroutine %d: SafeFromJSON failed: %v", id, err)
				return
			}
		}(i)
	}
	
	wg.Wait()
	close(errors)
	
	for err := range errors {
		t.Error(err)
	}
}

// TestMemoryPressure tests buffer pool efficiency under high memory pressure
func TestMemoryPressure(t *testing.T) {
	const numGoroutines = 100
	const numIterations = 1000
	
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	
	// Measure memory before
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < numIterations; j++ {
				// Create test data that will trigger buffer pool usage
				data := make([]string, 10)
				for k := range data {
					data[k] = fmt.Sprintf("item-%d-%d-%d", id, j, k)
				}
				
				// This should use the buffer pool
				result := StringJoinNonEmpty(data, ",")
				if len(result) == 0 {
					t.Errorf("goroutine %d: StringJoinNonEmpty returned empty", id)
					return
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// Measure memory after
	runtime.GC()
	runtime.ReadMemStats(&m2)
	
	// Check that memory usage is reasonable (this is a rough check)
	allocDiff := m2.TotalAlloc - m1.TotalAlloc
	if allocDiff > 100*1024*1024 { // 100MB threshold
		t.Logf("Memory allocation seems high: %d bytes", allocDiff)
		// This is not a failure, just a warning
	}
}

// TestErrorHandling tests proper error handling in concurrent scenarios
func TestErrorHandling(t *testing.T) {
	const numGoroutines = 20
	
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	
	errors := make(chan error, numGoroutines*4)
	
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			
			// Test RandomString with invalid inputs
			if _, err := RandomString(-1); err == nil {
				errors <- fmt.Errorf("goroutine %d: RandomString(-1) should return error", id)
			}
			
			if _, err := RandomString(2000); err == nil {
				errors <- fmt.Errorf("goroutine %d: RandomString(2000) should return error", id)
			}
			
			// Test RandomHex with invalid inputs
			if _, err := RandomHex(-1); err == nil {
				errors <- fmt.Errorf("goroutine %d: RandomHex(-1) should return error", id)
			}
			
			if _, err := RandomHex(1000); err == nil {
				errors <- fmt.Errorf("goroutine %d: RandomHex(1000) should return error", id)
			}
			
			// Test SafeFromJSON with oversized input
			oversized := string(make([]byte, 11*1024*1024)) // 11MB
			var target map[string]interface{}
			if err := SafeFromJSON(oversized, &target); err == nil {
				errors <- fmt.Errorf("goroutine %d: SafeFromJSON with oversized input should return error", id)
			}
		}(i)
	}
	
	wg.Wait()
	close(errors)
	
	for err := range errors {
		t.Error(err)
	}
}

// BenchmarkConcurrentStringJoin benchmarks the concurrent StringJoinNonEmpty
func BenchmarkConcurrentStringJoin(b *testing.B) {
	const numGoroutines = 10
	data := make([]string, 100)
	for i := range data {
		data[i] = fmt.Sprintf("item-%d", i)
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(numGoroutines)
		
		for j := 0; j < numGoroutines; j++ {
			go func() {
				defer wg.Done()
				StringJoinNonEmpty(data, ",")
			}()
		}
		
		wg.Wait()
	}
}

// BenchmarkBufferPool benchmarks the buffer pool efficiency
func BenchmarkBufferPool(b *testing.B) {
	data := make([]string, 50)
	for i := range data {
		data[i] = fmt.Sprintf("item-%d", i)
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		StringJoinNonEmpty(data, ",")
	}
}
