// Package bloomfilter provides a space-efficient probabilistic data structure
// for testing set membership with a configurable false positive rate.
package bloomfilter

import (
	"hash/fnv"
	"math"
	"sync"
)

// BloomFilter is a probabilistic data structure for set membership testing.
// It may return false positives but never false negatives.
// Safe for concurrent use.
type BloomFilter struct {
	bits   []uint64
	size   uint64
	hashes uint64
	salt   []byte
	mu     sync.RWMutex
}

// New creates a Bloom filter optimized for n elements with target false positive rate p.
func New(n uint64, p float64) *BloomFilter {
	// Optimal size: m = -n * ln(p) / (ln(2)^2)
	size := uint64(-float64(n) * math.Log(p) / (math.Ln2 * math.Ln2))
	if size == 0 {
		size = 1
	}
	// Optimal number of hash functions: k = (m/n) * ln(2)
	hashes := uint64(float64(size) / float64(n) * math.Ln2)
	if hashes == 0 {
		hashes = 1
	}
	// Round size up to nearest 64 for efficient bit storage
	words := (size + 63) / 64

	return &BloomFilter{
		bits:   make([]uint64, words),
		size:   words * 64,
		hashes: hashes,
	}
}

// Add inserts an element into the filter. Safe for concurrent use.
func (bf *BloomFilter) Add(data []byte) {
	positions := bf.positions(data)

	bf.mu.Lock()
	defer bf.mu.Unlock()

	for _, pos := range positions {
		word, bit := pos/64, pos%64
		bf.bits[word] |= 1 << bit
	}
}

// Contains checks if an element might be in the set.
// Returns true if probably present, false if definitely not present.
// Safe for concurrent use.
func (bf *BloomFilter) Contains(data []byte) bool {
	positions := bf.positions(data)

	bf.mu.RLock()
	defer bf.mu.RUnlock()

	for _, pos := range positions {
		word, bit := pos/64, pos%64
		if bf.bits[word]&(1<<bit) == 0 {
			return false
		}
	}
	return true
}

// Clear resets the filter, removing all elements. Safe for concurrent use.
func (bf *BloomFilter) Clear() {
	bf.mu.Lock()
	defer bf.mu.Unlock()

	for i := range bf.bits {
		bf.bits[i] = 0
	}
}

// positions generates hash positions for the given data using double hashing.
func (bf *BloomFilter) positions(data []byte) []uint64 {
	h1, h2 := bf.hash(data)
	positions := make([]uint64, bf.hashes)
	for i := uint64(0); i < bf.hashes; i++ {
		// Double hashing: (h1 + i*h2) % size
		positions[i] = (h1 + i*h2) % bf.size
	}
	return positions
}

// hash computes two hash values from the data using FNV variants.
// Creates new hashers locally for thread safety.
func (bf *BloomFilter) hash(data []byte) (uint64, uint64) {
	h1 := fnv.New64a()
	h2 := fnv.New64()

	h1.Write(data)
	h2.Write(data)

	return h1.Sum64(), h2.Sum64()
}

// Size returns the total number of bits in the filter.
func (bf *BloomFilter) Size() uint64 {
	return bf.size
}

// HashCount returns the number of hash functions used.
func (bf *BloomFilter) HashCount() uint64 {
	return bf.hashes
}
