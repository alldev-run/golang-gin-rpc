package configcenter

import (
	"context"
	"path"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdProvider implements Provider backed by an etcd cluster.
type EtcdProvider struct {
	client *clientv3.Client
	prefix string
}

// EtcdOption configures EtcdProvider.
type EtcdOption func(*EtcdProvider)

// WithEtcdPrefix overrides the default key prefix ("/configcenter").
func WithEtcdPrefix(prefix string) EtcdOption {
	return func(p *EtcdProvider) {
		p.prefix = prefix
	}
}

// NewEtcdProvider creates an EtcdProvider using the supplied client config.
func NewEtcdProvider(cfg clientv3.Config, opts ...EtcdOption) (*EtcdProvider, error) {
	client, err := clientv3.New(cfg)
	if err != nil {
		return nil, err
	}

	provider := &EtcdProvider{
		client: client,
		prefix: "/configcenter",
	}

	for _, opt := range opts {
		opt(provider)
	}

	return provider, nil
}

func (p *EtcdProvider) buildKey(namespace, key string) string {
	return path.Join(p.prefix, namespace, key)
}

// Get retrieves the configuration value stored at namespace/key.
func (p *EtcdProvider) Get(ctx context.Context, namespace, key string) ([]byte, int64, error) {
	resp, err := p.client.Get(ctx, p.buildKey(namespace, key))
	if err != nil {
		return nil, 0, err
	}
	if len(resp.Kvs) == 0 {
		return nil, 0, ErrNotFound
	}
	kv := resp.Kvs[0]
	return append([]byte(nil), kv.Value...), kv.ModRevision, nil
}

// Set writes the configuration value.
func (p *EtcdProvider) Set(ctx context.Context, namespace, key string, value []byte, metadata map[string]string) (int64, error) {
	resp, err := p.client.Put(ctx, p.buildKey(namespace, key), string(value))
	if err != nil {
		return 0, err
	}
	return resp.Header.Revision, nil
}

// Delete removes the configuration key.
func (p *EtcdProvider) Delete(ctx context.Context, namespace, key string) error {
	resp, err := p.client.Delete(ctx, p.buildKey(namespace, key))
	if err != nil {
		return err
	}
	if resp.Deleted == 0 {
		return ErrNotFound
	}
	return nil
}

// Watch streams configuration changes for a namespace.
func (p *EtcdProvider) Watch(ctx context.Context, namespace string) (<-chan ConfigChange, error) {
	out := make(chan ConfigChange, 32)
	prefix := path.Join(p.prefix, namespace)

	go func() {
		watchChan := p.client.Watch(ctx, prefix, clientv3.WithPrefix())
		for {
			sel := ctx.Done()
			select {
			case <-sel:
				close(out)
				return
			case resp, ok := <-watchChan:
				if !ok {
					close(out)
					return
				}
				for _, ev := range resp.Events {
					change := ConfigChange{
						Namespace: namespace,
						Key:       extractKey(prefix, string(ev.Kv.Key)),
						Version:   ev.Kv.ModRevision,
						Timestamp: time.Now(),
					}
					if ev.Type == clientv3.EventTypeDelete {
						change.Change = ChangeTypeDelete
					} else {
						change.Change = ChangeTypeSet
						change.Value = append([]byte(nil), ev.Kv.Value...)
					}
					select {
					case out <- change:
					case <-ctx.Done():
						close(out)
						return
					}
				}
			}
		}
	}()

	return out, nil
}

func extractKey(prefix, full string) string {
	trimmed := strings.TrimPrefix(full, prefix)
	trimmed = strings.TrimPrefix(trimmed, "/")
	return trimmed
}

// Close shuts down the etcd client.
func (p *EtcdProvider) Close() error {
	return p.client.Close()
}
