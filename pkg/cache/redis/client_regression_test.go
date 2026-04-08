package redis

import (
	"testing"

	"github.com/redis/go-redis/v9"
)

// TestGetClientForKey_BoundsChecking verifies that getClientForKey handles
// out-of-bounds index safely by falling back to available nodes.
func TestGetClientForKey_BoundsChecking(t *testing.T) {
	// Create client with multi-mode but simulate misconfigured KeyPrefixRoutes
	client := &Client{
		config: Config{
			Mode: ModeMulti,
			Multi: struct {
				ShardingStrategy string         `yaml:"sharding_strategy" json:"sharding_strategy"`
				KeyPrefixRoutes  map[string]int `yaml:"key_prefix_routes" json:"key_prefix_routes"`
				DefaultNode      int            `yaml:"default_node" json:"default_node"`
			}{
				ShardingStrategy: "hash",
				KeyPrefixRoutes: map[string]int{
					"invalid:": 99, // Out of bounds index
				},
				DefaultNode: 0,
			},
		},
		nodes: []*redis.Client{nil}, // One nil node
	}

	// Should not panic and should return nil (no available nodes)
	result := client.getClientForKey("invalid:key")
	// Since all nodes are nil, result should be nil
	if result != nil {
		t.Errorf("expected nil client for invalid configuration, got %v", result)
	}
}

// TestGetNodeForKey_OutOfBoundsDefaultNode verifies safe handling of
// out-of-bounds DefaultNode configuration.
func TestGetNodeForKey_OutOfBoundsDefaultNode(t *testing.T) {
	client := &Client{
		config: Config{
			Mode: ModeMulti,
			Multi: struct {
				ShardingStrategy string         `yaml:"sharding_strategy" json:"sharding_strategy"`
				KeyPrefixRoutes  map[string]int `yaml:"key_prefix_routes" json:"key_prefix_routes"`
				DefaultNode      int            `yaml:"default_node" json:"default_node"`
			}{
				ShardingStrategy: "range",
				DefaultNode:      10, // Out of bounds
			},
		},
		nodes: []*redis.Client{nil, nil}, // 2 nodes, but DefaultNode=10
	}

	idx := client.getNodeForKey("anykey")
	if idx != 0 {
		t.Errorf("expected index 0 for out-of-bounds default, got %d", idx)
	}
}

// TestGetNodeForKey_HashWithEmptyNodes verifies hash sharding handles empty nodes.
func TestGetNodeForKey_HashWithEmptyNodes(t *testing.T) {
	client := &Client{
		config: Config{
			Mode: ModeMulti,
			Multi: struct {
				ShardingStrategy string         `yaml:"sharding_strategy" json:"sharding_strategy"`
				KeyPrefixRoutes  map[string]int `yaml:"key_prefix_routes" json:"key_prefix_routes"`
				DefaultNode      int            `yaml:"default_node" json:"default_node"`
			}{
				ShardingStrategy: "hash",
			},
		},
		nodes: []*redis.Client{}, // Empty nodes
	}

	idx := client.getNodeForKey("testkey")
	if idx != 0 {
		t.Errorf("expected index 0 for empty nodes, got %d", idx)
	}
}

// TestGetSafeDefaultNode_BoundsChecking verifies getSafeDefaultNode handles
// various edge cases correctly.
func TestGetSafeDefaultNode_BoundsChecking(t *testing.T) {
	tests := []struct {
		name       string
		nodes      []*redis.Client
		defaultIdx int
		expected   int
	}{
		{
			name:       "empty nodes",
			nodes:      []*redis.Client{},
			defaultIdx: 0,
			expected:   0,
		},
		{
			name:       "negative default index",
			nodes:      []*redis.Client{nil, nil},
			defaultIdx: -1,
			expected:   0,
		},
		{
			name:       "out of bounds default index",
			nodes:      []*redis.Client{nil, nil},
			defaultIdx: 5,
			expected:   0,
		},
		{
			name:       "valid default index",
			nodes:      []*redis.Client{nil, nil, nil},
			defaultIdx: 1,
			expected:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				config: Config{
					Mode: ModeMulti,
					Multi: struct {
						ShardingStrategy string         `yaml:"sharding_strategy" json:"sharding_strategy"`
						KeyPrefixRoutes  map[string]int `yaml:"key_prefix_routes" json:"key_prefix_routes"`
						DefaultNode      int            `yaml:"default_node" json:"default_node"`
					}{
						DefaultNode: tt.defaultIdx,
					},
				},
				nodes: tt.nodes,
			}

			result := client.getSafeDefaultNode()
			if result != tt.expected {
				t.Errorf("getSafeDefaultNode() = %d, expected %d", result, tt.expected)
			}
		})
	}
}

// TestGetFirstAvailableNode verifies the fallback mechanism works correctly.
func TestGetFirstAvailableNode(t *testing.T) {
	tests := []struct {
		name     string
		nodes    []*redis.Client
		expected *redis.Client
	}{
		{
			name:     "all nil nodes",
			nodes:    []*redis.Client{nil, nil, nil},
			expected: nil,
		},
		{
			name:     "first non-nil at start",
			nodes:    []*redis.Client{&redis.Client{}, nil, nil},
			expected: &redis.Client{},
		},
		{
			name:     "first non-nil in middle",
			nodes:    []*redis.Client{nil, &redis.Client{}, nil},
			expected: &redis.Client{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{nodes: tt.nodes}

			result := client.getFirstAvailableNode()
			if tt.expected == nil {
				if result != nil {
					t.Errorf("getFirstAvailableNode() = %v, expected nil", result)
				}
			} else {
				if result == nil {
					t.Errorf("getFirstAvailableNode() = nil, expected non-nil")
				}
			}
		})
	}
}
