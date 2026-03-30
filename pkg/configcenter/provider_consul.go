package configcenter

import (
	"context"
	"path"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
)

// ConsulProvider implements Provider backed by Consul KV.
type ConsulProvider struct {
	client       *api.Client
	prefix       string
	pollInterval time.Duration
}

// ConsulOption configures ConsulProvider.
type ConsulOption func(*ConsulProvider)

// WithConsulPrefix overrides the default key prefix ("configcenter").
func WithConsulPrefix(prefix string) ConsulOption {
	return func(p *ConsulProvider) {
		p.prefix = strings.Trim(prefix, "/")
	}
}

// WithConsulPollInterval sets polling interval used by Watch.
func WithConsulPollInterval(interval time.Duration) ConsulOption {
	return func(p *ConsulProvider) {
		if interval > 0 {
			p.pollInterval = interval
		}
	}
}

// NewConsulProvider creates a ConsulProvider from consul api config.
func NewConsulProvider(cfg *api.Config, opts ...ConsulOption) (*ConsulProvider, error) {
	if cfg == nil {
		cfg = api.DefaultConfig()
	}
	client, err := api.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	p := &ConsulProvider{
		client:       client,
		prefix:       "configcenter",
		pollInterval: 2 * time.Second,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p, nil
}

func (p *ConsulProvider) buildKey(namespace, key string) string {
	return path.Join(p.prefix, namespace, key)
}

// Get fetches a config value from Consul KV.
func (p *ConsulProvider) Get(ctx context.Context, namespace, key string) ([]byte, int64, error) {
	queryOpts := (&api.QueryOptions{}).WithContext(ctx)
	pair, _, err := p.client.KV().Get(p.buildKey(namespace, key), queryOpts)
	if err != nil {
		return nil, 0, err
	}
	if pair == nil {
		return nil, 0, ErrNotFound
	}
	return append([]byte(nil), pair.Value...), int64(pair.ModifyIndex), nil
}

// Set stores a config value in Consul KV.
func (p *ConsulProvider) Set(ctx context.Context, namespace, key string, value []byte, metadata map[string]string) (int64, error) {
	pair := &api.KVPair{
		Key:   p.buildKey(namespace, key),
		Value: append([]byte(nil), value...),
	}
	writeOpts := (&api.WriteOptions{}).WithContext(ctx)
	if _, err := p.client.KV().Put(pair, writeOpts); err != nil {
		return 0, err
	}

	queryOpts := (&api.QueryOptions{}).WithContext(ctx)
	stored, _, err := p.client.KV().Get(pair.Key, queryOpts)
	if err != nil {
		return 0, err
	}
	if stored == nil {
		return 0, ErrNotFound
	}
	return int64(stored.ModifyIndex), nil
}

// Delete removes a config value from Consul KV.
func (p *ConsulProvider) Delete(ctx context.Context, namespace, key string) error {
	fullKey := p.buildKey(namespace, key)
	queryOpts := (&api.QueryOptions{}).WithContext(ctx)
	pair, _, err := p.client.KV().Get(fullKey, queryOpts)
	if err != nil {
		return err
	}
	if pair == nil {
		return ErrNotFound
	}
	writeOpts := (&api.WriteOptions{}).WithContext(ctx)
	_, err = p.client.KV().Delete(fullKey, writeOpts)
	return err
}

// Watch polls Consul KV and emits change events for the namespace.
func (p *ConsulProvider) Watch(ctx context.Context, namespace string) (<-chan ConfigChange, error) {
	out := make(chan ConfigChange, 32)
	prefix := path.Join(p.prefix, namespace) + "/"

	go func() {
		defer close(out)

		last := map[string]*api.KVPair{}
		ticker := time.NewTicker(p.pollInterval)
		defer ticker.Stop()

		for {
			current, err := p.listNamespace(ctx, prefix)
			if err == nil {
				changes := diffConsulPairs(namespace, prefix, last, current)
				for _, ch := range changes {
					select {
					case out <- ch:
					case <-ctx.Done():
						return
					}
				}
				last = current
			}

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	return out, nil
}

func (p *ConsulProvider) listNamespace(ctx context.Context, prefix string) (map[string]*api.KVPair, error) {
	queryOpts := (&api.QueryOptions{}).WithContext(ctx)
	pairs, _, err := p.client.KV().List(prefix, queryOpts)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*api.KVPair, len(pairs))
	for _, pair := range pairs {
		cp := *pair
		cp.Value = append([]byte(nil), pair.Value...)
		result[cp.Key] = &cp
	}
	return result, nil
}

func diffConsulPairs(namespace, prefix string, oldPairs, newPairs map[string]*api.KVPair) []ConfigChange {
	changes := make([]ConfigChange, 0)

	for key, newPair := range newPairs {
		oldPair, ok := oldPairs[key]
		if !ok || oldPair.ModifyIndex != newPair.ModifyIndex {
			changes = append(changes, ConfigChange{
				Namespace: namespace,
				Key:       extractKey(prefix, key),
				Value:     append([]byte(nil), newPair.Value...),
				Version:   int64(newPair.ModifyIndex),
				Change:    ChangeTypeSet,
				Timestamp: time.Now(),
			})
		}
	}

	for key, oldPair := range oldPairs {
		if _, ok := newPairs[key]; !ok {
			changes = append(changes, ConfigChange{
				Namespace: namespace,
				Key:       extractKey(prefix, key),
				Version:   int64(oldPair.ModifyIndex),
				Change:    ChangeTypeDelete,
				Timestamp: time.Now(),
			})
		}
	}

	return changes
}

// Close closes provider resources.
func (p *ConsulProvider) Close() error {
	return nil
}
