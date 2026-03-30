package configcenter

import (
	"context"
	"sync"
	"sync/atomic"
)

// MemoryProvider is an in-memory Provider implementation used for tests and examples.
type MemoryProvider struct {
	mu       sync.RWMutex
	data     map[string]map[string]*memValue
	watchers map[string][]chan ConfigChange
	closed   atomic.Bool
	version  int64
}

type memValue struct {
	value   []byte
	version int64
}

// NewMemoryProvider creates a new MemoryProvider instance.
func NewMemoryProvider() *MemoryProvider {
	return &MemoryProvider{
		data:     make(map[string]map[string]*memValue),
		watchers: make(map[string][]chan ConfigChange),
	}
}

// Get implements Provider.
func (p *MemoryProvider) Get(ctx context.Context, namespace, key string) ([]byte, int64, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if ns, ok := p.data[namespace]; ok {
		if val, ok := ns[key]; ok {
			return append([]byte(nil), val.value...), val.version, nil
		}
	}
	return nil, 0, ErrNotFound
}

// Set implements Provider.
func (p *MemoryProvider) Set(ctx context.Context, namespace, key string, value []byte, metadata map[string]string) (int64, error) {
	p.mu.Lock()
	if p.data[namespace] == nil {
		p.data[namespace] = make(map[string]*memValue)
	}
	ver := atomic.AddInt64(&p.version, 1)
	p.data[namespace][key] = &memValue{value: append([]byte(nil), value...), version: ver}
	p.mu.Unlock()

	p.notify(ConfigChange{
		Namespace: namespace,
		Key:       key,
		Value:     append([]byte(nil), value...),
		Version:   ver,
		Change:    ChangeTypeSet,
		Metadata:  metadata,
	})

	return ver, nil
}

// Delete implements Provider.
func (p *MemoryProvider) Delete(ctx context.Context, namespace, key string) error {
	p.mu.Lock()
	if ns, ok := p.data[namespace]; ok {
		if _, ok := ns[key]; ok {
			delete(ns, key)
			if len(ns) == 0 {
				delete(p.data, namespace)
			}
			p.mu.Unlock()
			p.notify(ConfigChange{
				Namespace: namespace,
				Key:       key,
				Change:    ChangeTypeDelete,
			})
			return nil
		}
	}
	p.mu.Unlock()
	return ErrNotFound
}

// Watch implements Provider.
func (p *MemoryProvider) Watch(ctx context.Context, namespace string) (<-chan ConfigChange, error) {
	ch := make(chan ConfigChange, 16)

	p.mu.Lock()
	p.watchers[namespace] = append(p.watchers[namespace], ch)
	p.mu.Unlock()

	go func() {
		<-ctx.Done()
		p.mu.Lock()
		watchers := p.watchers[namespace]
		for i, c := range watchers {
			if c == ch {
				p.watchers[namespace] = append(watchers[:i], watchers[i+1:]...)
				break
			}
		}
		if len(p.watchers[namespace]) == 0 {
			delete(p.watchers, namespace)
		}
		p.mu.Unlock()
	}()

	return ch, nil
}

// Close implements Provider.
func (p *MemoryProvider) Close() error {
	if p.closed.Swap(true) {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for ns := range p.watchers {
		delete(p.watchers, ns)
	}
	return nil
}

func (p *MemoryProvider) notify(change ConfigChange) {
	p.mu.RLock()
	watchers := append([]chan ConfigChange(nil), p.watchers[change.Namespace]...)
	p.mu.RUnlock()

	for _, ch := range watchers {
		select {
		case ch <- change:
		default:
		}
	}
}
