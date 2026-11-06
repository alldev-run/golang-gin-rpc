package router

import (
	"sync"
	"testing"
)

func TestRouterFactoryConcurrentAccess(t *testing.T) {
	original := GetRouterFactory()
	t.Cleanup(func() { SetRouterFactory(original) })

	replacement := NewRouterFactory()
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			SetRouterFactory(replacement)
		}()
		go func() {
			defer wg.Done()
			if GetRouterFactory() == nil {
				t.Error("GetRouterFactory() returned nil")
			}
		}()
	}
	wg.Wait()

	if GetRouterFactory() != replacement {
		t.Fatal("GetRouterFactory() did not return the factory set by SetRouterFactory")
	}
}
