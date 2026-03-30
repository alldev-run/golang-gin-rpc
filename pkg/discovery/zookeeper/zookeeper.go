package zookeeper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/go-zookeeper/zk"
)

type ServiceInstance struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Address string            `json:"address"`
	Port    int               `json:"port"`
	Payload map[string]string `json:"payload"`
}

type Registry struct {
	conn     *zk.Conn
	basePath string
	mu       sync.RWMutex
	paths    map[string]string
}

func NewRegistry(addr string, timeout time.Duration, options map[string]interface{}) (*Registry, error) {
	servers := splitServers(addr)
	if len(servers) == 0 {
		return nil, fmt.Errorf("zookeeper address is required")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	basePath := "/services"
	if options != nil {
		if value, ok := options["base_path"].(string); ok && strings.TrimSpace(value) != "" {
			basePath = value
		}
	}
	conn, _, err := zk.Connect(servers, timeout)
	if err != nil {
		return nil, err
	}
	registry := &Registry{
		conn:     conn,
		basePath: normalizePath(basePath),
		paths:    make(map[string]string),
	}
	if err := registry.ensurePath(registry.basePath); err != nil {
		conn.Close()
		return nil, err
	}
	return registry, nil
}

func (r *Registry) Register(ctx context.Context, inst *ServiceInstance) error {
	if r == nil || r.conn == nil {
		return fmt.Errorf("zookeeper registry is not initialized")
	}
	if inst == nil {
		return fmt.Errorf("service instance is nil")
	}
	servicePath := path.Join(r.basePath, inst.Name)
	if err := r.ensurePath(servicePath); err != nil {
		return err
	}
	fullPath := path.Join(servicePath, inst.ID)
	payload, err := json.Marshal(inst)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if oldPath, ok := r.paths[inst.ID]; ok {
		_ = r.conn.Delete(oldPath, -1)
		delete(r.paths, inst.ID)
	}
	if exists, _, err := r.conn.Exists(fullPath); err != nil {
		return err
	} else if exists {
		if err := r.conn.Delete(fullPath, -1); err != nil && err != zk.ErrNoNode {
			return err
		}
	}
	createdPath, err := r.conn.Create(fullPath, payload, zk.FlagEphemeral, zk.WorldACL(zk.PermAll))
	if err != nil {
		return err
	}
	r.paths[inst.ID] = createdPath
	return nil
}

func (r *Registry) Deregister(ctx context.Context, inst *ServiceInstance) error {
	if r == nil || r.conn == nil || inst == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	fullPath := r.paths[inst.ID]
	if fullPath == "" {
		fullPath = path.Join(r.basePath, inst.Name, inst.ID)
	}
	if err := r.conn.Delete(fullPath, -1); err != nil && err != zk.ErrNoNode {
		return err
	}
	delete(r.paths, inst.ID)
	return nil
}

func (r *Registry) GetService(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
	if r == nil || r.conn == nil {
		return nil, fmt.Errorf("zookeeper registry is not initialized")
	}
	servicePath := path.Join(r.basePath, serviceName)
	children, _, err := r.conn.Children(servicePath)
	if err != nil {
		if err == zk.ErrNoNode {
			return []*ServiceInstance{}, nil
		}
		return nil, err
	}
	instances := make([]*ServiceInstance, 0, len(children))
	for _, child := range children {
		fullPath := path.Join(servicePath, child)
		data, _, err := r.conn.Get(fullPath)
		if err != nil {
			if err == zk.ErrNoNode {
				continue
			}
			return nil, err
		}
		var inst ServiceInstance
		if err := json.Unmarshal(data, &inst); err != nil {
			continue
		}
		instances = append(instances, &inst)
	}
	return instances, nil
}

func (r *Registry) Close() error {
	if r == nil || r.conn == nil {
		return nil
	}
	r.conn.Close()
	return nil
}

func (r *Registry) ensurePath(fullPath string) error {
	parts := strings.Split(strings.Trim(normalizePath(fullPath), "/"), "/")
	current := ""
	for _, part := range parts {
		current += "/" + part
		exists, _, err := r.conn.Exists(current)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		_, err = r.conn.Create(current, nil, 0, zk.WorldACL(zk.PermAll))
		if err != nil && !errors.Is(err, zk.ErrNodeExists) {
			return err
		}
	}
	return nil
}

func splitServers(addr string) []string {
	items := strings.Split(addr, ",")
	servers := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			servers = append(servers, trimmed)
		}
	}
	return servers
}

func normalizePath(value string) string {
	if strings.TrimSpace(value) == "" {
		return "/services"
	}
	if !strings.HasPrefix(value, "/") {
		return "/" + value
	}
	return value
}
