package bloomfilter

import (
	"testing"
)

func TestBloomFilter(t *testing.T) {
	// Create filter for 10000 elements with 1% false positive rate
	bf := New(10000, 0.01)

	// Add elements
	elements := [][]byte{
		[]byte("hello"),
		[]byte("world"),
		[]byte("bloom"),
		[]byte("filter"),
	}

	for _, elem := range elements {
		bf.Add(elem)
	}

	// Check all added elements are found (no false negatives)
	for _, elem := range elements {
		if !bf.Contains(elem) {
			t.Errorf("Expected to find %s", string(elem))
		}
	}

	// Check a definitely non-existent element
	// This might yield false positive, but unlikely with proper sizing
	if bf.Contains([]byte("definitely-not-there")) {
		t.Log("False positive occurred (this is expected occasionally)")
	}
}

func TestBloomFilterClear(t *testing.T) {
	bf := New(1000, 0.01)
	bf.Add([]byte("test"))

	if !bf.Contains([]byte("test")) {
		t.Error("Should contain 'test' before clear")
	}

	bf.Clear()

	if bf.Contains([]byte("test")) {
		t.Error("Should not contain 'test' after clear")
	}
}

func TestFalsePositiveRate(t *testing.T) {
	// Create filter with 1% target false positive rate
	n := uint64(10000)
	p := 0.01
	bf := New(n, p)

	// Add n elements
	for i := uint64(0); i < n; i++ {
		bf.Add([]byte(string(rune(i))))
	}

	// Test with different elements
	falsePositives := 0
	testCount := 10000
	for i := n; i < n+uint64(testCount); i++ {
		if bf.Contains([]byte(string(rune(i)))) {
			falsePositives++
		}
	}

	actualRate := float64(falsePositives) / float64(testCount)
	t.Logf("False positive rate: %.4f (target: %.4f)", actualRate, p)

	// Should be reasonably close to target (with some tolerance)
	if actualRate > p*3 {
		t.Errorf("False positive rate %.4f too high, expected ~%.4f", actualRate, p)
	}
}
