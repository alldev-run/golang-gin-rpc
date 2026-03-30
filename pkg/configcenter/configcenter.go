package configcenter

import (
	"context"
	"fmt"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// cacheEntry stores cached configuration.
type cacheEntry struct {
	value   []byte
	version int64
	expires time.Time
}

// ConfigCenter orchestrates configuration operations and change propagation.
type ConfigCenter struct {
	provider Provider

	cache    map[string]map[string]*cacheEntry
	cacheTTL time.Duration
	cacheMu  sync.RWMutex

	subsMu   sync.Mutex
	subs     map[string][]*subscription
	watchers map[string]context.CancelFunc

	closed atomic.Bool

	log Logger
}

// subscription implements Subscription backed by an event handler removal.
type subscription struct {
	closeOnce sync.Once
	cancel    context.CancelFunc
	handler   func(ConfigChange)
}

func (s *subscription) Close() {
	s.closeOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
	})
}

// New creates a ConfigCenter instance bound to the provider.
func New(provider Provider, opts ...Option) *ConfigCenter {
	cc := &ConfigCenter{
		provider: provider,
		cache:    make(map[string]map[string]*cacheEntry),
		subs:     make(map[string][]*subscription),
		watchers: make(map[string]context.CancelFunc),
		cacheTTL: 30 * time.Second,
		log:      func(args ...interface{}) {},
	}

	for _, opt := range opts {
		opt(cc)
	}

	return cc
}

func cacheKey(namespace, key string) string {
	return fmt.Sprintf("%s::%s", namespace, key)
}

// Get retrieves a configuration value, using cache when valid.
func (cc *ConfigCenter) Get(ctx context.Context, namespace, key string) ([]byte, int64, error) {
	if cc.closed.Load() {
		return nil, 0, errors.New("configcenter: closed")
	}
	if v, ver, ok := cc.getFromCache(namespace, key); ok {
		return v, ver, nil
	}

	value, version, err := cc.provider.Get(ctx, namespace, key)
	if err != nil {
		return nil, 0, err
	}
	cc.setCache(namespace, key, value, version)
	return value, version, nil
}

// Set stores a configuration value and updates the cache.
func (cc *ConfigCenter) Set(ctx context.Context, namespace, key string, value []byte, metadata map[string]string) (int64, error) {
	if cc.closed.Load() {
		return 0, errors.New("configcenter: closed")
	}
	version, err := cc.provider.Set(ctx, namespace, key, value, metadata)
	if err != nil {
		return 0, err
	}
	cc.setCache(namespace, key, value, version)
	return version, nil
}

// Delete removes a configuration key and updates cache.
func (cc *ConfigCenter) Delete(ctx context.Context, namespace, key string) error {
	if cc.closed.Load() {
		return errors.New("configcenter: closed")
	}
	if err := cc.provider.Delete(ctx, namespace, key); err != nil {
		return err
	}
	cc.deleteCache(namespace, key)
	return nil
}

// Subscribe registers a callback for namespace changes.
func (cc *ConfigCenter) Subscribe(ctx context.Context, namespace string, handler func(ConfigChange)) (Subscription, error) {
	if cc.closed.Load() {
		return nil, errors.New("configcenter: closed")
	}
	if handler == nil {
		return nil, fmt.Errorf("configcenter: handler cannot be nil")
	}
	subCtx, cancel := context.WithCancel(context.Background())
	sub := &subscription{cancel: cancel, handler: handler}

	// cancel subscription when caller context is done
	go func() {
		select {
		case <-ctx.Done():
			cancel()
		case <-subCtx.Done():
		}
	}()

	cc.subsMu.Lock()
	cc.subs[namespace] = append(cc.subs[namespace], sub)
	startWatcher := len(cc.subs[namespace]) == 1
	if startWatcher {
		watchCtx, watchCancel := context.WithCancel(context.Background())
		cc.watchers[namespace] = watchCancel
		started := make(chan struct{})
		go cc.watchNamespace(watchCtx, namespace, started)
		cc.subsMu.Unlock()
		<-started
		goto subscribed
	}
	cc.subsMu.Unlock()

subscribed:

	go func() {
		<-subCtx.Done()
		cc.removeSubscription(namespace, sub)
	}()

	return sub, nil
}

func (cc *ConfigCenter) removeSubscription(namespace string, sub *subscription) {
	cc.subsMu.Lock()
	defer cc.subsMu.Unlock()

	subs := cc.subs[namespace]
	for i, s := range subs {
		if s == sub {
			cc.subs[namespace] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	if len(cc.subs[namespace]) == 0 {
		delete(cc.subs, namespace)
		if cancel, ok := cc.watchers[namespace]; ok {
			cancel()
			delete(cc.watchers, namespace)
		}
	}
}

func (cc *ConfigCenter) watchNamespace(ctx context.Context, namespace string, started chan<- struct{}) {
	startedOnce := sync.Once{}
	markStarted := func() {
		if started == nil {
			return
		}
		startedOnce.Do(func() {
			close(started)
		})
	}

	for {
		if ctx.Err() != nil {
			markStarted()
			cc.log("configcenter: watch stopped", namespace)
			return
		}

		ch, err := cc.provider.Watch(ctx, namespace)
		if err != nil {
			markStarted()
			cc.log("configcenter: watch error", err)
			select {
			case <-time.After(500 * time.Millisecond):
				continue
			case <-ctx.Done():
				return
			}
		}

		markStarted()

		retryWatch := false
		for {
			select {
			case <-ctx.Done():
				markStarted()
				cc.log("configcenter: watch stopped", namespace)
				return
			case change, ok := <-ch:
				if !ok {
					cc.log("configcenter: watch channel closed, retrying", namespace)
					select {
					case <-time.After(200 * time.Millisecond):
					case <-ctx.Done():
						return
					}
					retryWatch = true
					break
				}
				cc.handleChange(change)
			}
			if retryWatch {
				break
			}
		}
	}
}

func (cc *ConfigCenter) handleChange(change ConfigChange) {
	if change.Change == ChangeTypeDelete {
		cc.deleteCache(change.Namespace, change.Key)
	} else {
		cc.setCache(change.Namespace, change.Key, change.Value, change.Version)
	}

	cc.subsMu.Lock()
	subs := append([]*subscription(nil), cc.subs[change.Namespace]...)
	cc.subsMu.Unlock()

	for _, sub := range subs {
		s := sub
		if s.handler == nil {
			continue
		}
		go s.handler(change)
	}
}

func (cc *ConfigCenter) getFromCache(namespace, key string) ([]byte, int64, bool) {
	cc.cacheMu.RLock()
	defer cc.cacheMu.RUnlock()

	keys, ok := cc.cache[namespace]
	if !ok {
		return nil, 0, false
	}
	entry, ok := keys[key]
	if !ok || time.Now().After(entry.expires) {
		return nil, 0, false
	}
	return entry.value, entry.version, true
}

func (cc *ConfigCenter) setCache(namespace, key string, value []byte, version int64) {
	cc.cacheMu.Lock()
	defer cc.cacheMu.Unlock()

	if cc.cache[namespace] == nil {
		cc.cache[namespace] = make(map[string]*cacheEntry)
	}
	cc.cache[namespace][key] = &cacheEntry{
		value:   append([]byte(nil), value...),
		version: version,
		expires: time.Now().Add(cc.cacheTTL),
	}
}

func (cc *ConfigCenter) deleteCache(namespace, key string) {
	cc.cacheMu.Lock()
	defer cc.cacheMu.Unlock()

	if cc.cache[namespace] == nil {
		return
	}
	delete(cc.cache[namespace], key)
	if len(cc.cache[namespace]) == 0 {
		delete(cc.cache, namespace)
	}
}

// Close releases resources and closes provider.
func (cc *ConfigCenter) Close() error {
	if cc.closed.Swap(true) {
		return nil
	}

	cc.subsMu.Lock()
	for namespace, cancel := range cc.watchers {
		cancel()
		delete(cc.watchers, namespace)
	}
	for namespace, subs := range cc.subs {
		for _, sub := range subs {
			sub.Close()
		}
		delete(cc.subs, namespace)
	}
	cc.subsMu.Unlock()

	return cc.provider.Close()
}
