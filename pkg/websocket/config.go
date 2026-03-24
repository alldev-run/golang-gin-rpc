package websocket

import (
	"fmt"
	"strings"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/messaging"
)

type NodeConfig struct {
	Enabled            bool          `yaml:"enabled" json:"enabled"`
	NodeID             string        `yaml:"node_id" json:"node_id"`
	Name               string        `yaml:"name" json:"name"`
	Host               string        `yaml:"host" json:"host"`
	Port               int           `yaml:"port" json:"port"`
	Path               string        `yaml:"path" json:"path"`
	ReadTimeout        time.Duration `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout       time.Duration `yaml:"write_timeout" json:"write_timeout"`
	HeartbeatTimeout   time.Duration `yaml:"heartbeat_timeout" json:"heartbeat_timeout"`
	IdleTimeout        time.Duration `yaml:"idle_timeout" json:"idle_timeout"`
	RequireAuth        bool          `yaml:"require_auth" json:"require_auth"`
}

type ClusterOptions struct {
	GroupBroadcast      bool `yaml:"group_broadcast" json:"group_broadcast"`
	UserBroadcast       bool `yaml:"user_broadcast" json:"user_broadcast"`
	ClientBroadcast     bool `yaml:"client_broadcast" json:"client_broadcast"`
	ConnectionTargeting bool `yaml:"connection_targeting" json:"connection_targeting"`
}

type ClusterTransportConfig struct {
	Type      string                 `yaml:"type" json:"type"`
	Topic     string                 `yaml:"topic" json:"topic"`
	Enabled   bool                   `yaml:"enabled" json:"enabled"`
	Messaging messaging.Config       `yaml:"messaging" json:"messaging"`
	Options   map[string]interface{} `yaml:"options" json:"options"`
}

type ClusterRuntimeConfig struct {
	Enabled   bool                   `yaml:"enabled" json:"enabled"`
	NodeID    string                 `yaml:"node_id" json:"node_id"`
	Topic     string                 `yaml:"topic" json:"topic"`
	Transport ClusterTransportConfig `yaml:"transport" json:"transport"`
	Options   ClusterOptions         `yaml:"options" json:"options"`
}

type ConfigFile struct {
	Node    NodeConfig           `yaml:"node" json:"node"`
	Client  Config               `yaml:"client" json:"client"`
	Server  ServerConfig         `yaml:"server" json:"server"`
	Cluster ClusterRuntimeConfig `yaml:"cluster" json:"cluster"`
}

func DefaultNodeConfig() NodeConfig {
	return NodeConfig{
		Enabled:          true,
		Name:             "websocket-node",
		Host:             "0.0.0.0",
		Port:             18080,
		Path:             "/ws",
		ReadTimeout:      30 * time.Second,
		WriteTimeout:     30 * time.Second,
		HeartbeatTimeout: 90 * time.Second,
		IdleTimeout:      2 * time.Minute,
	}
}

func DefaultClusterTransportConfig() ClusterTransportConfig {
	return ClusterTransportConfig{
		Type:    "memory",
		Topic:   DefaultClusterConfig().Topic,
		Enabled: false,
		Messaging: messaging.Config{
			Type:      messaging.MessageTypeRabbitMQ,
			Host:      "localhost",
			Port:      5672,
			Username:  "guest",
			Password:  "guest",
			Database:  "/",
			Timeout:   30 * time.Second,
			Options:   map[string]interface{}{},
			Enabled:   true,
		},
		Options: map[string]interface{}{},
	}
}

func DefaultClusterRuntimeConfig() ClusterRuntimeConfig {
	return ClusterRuntimeConfig{
		Enabled: false,
		Topic:   DefaultClusterConfig().Topic,
		Transport: DefaultClusterTransportConfig(),
		Options: ClusterOptions{
			GroupBroadcast:      true,
			UserBroadcast:       true,
			ClientBroadcast:     true,
			ConnectionTargeting: true,
		},
	}
}

func DefaultConfigFile() ConfigFile {
	server := DefaultServerConfig()
	client := DefaultConfig()
	return ConfigFile{
		Node:    DefaultNodeConfig(),
		Client:  client,
		Server:  server,
		Cluster: DefaultClusterRuntimeConfig(),
	}
}

func (c *ConfigFile) Normalize() {
	if c.Node.Path == "" {
		c.Node.Path = "/ws"
	}
	if c.Server.Path == "" {
		c.Server.Path = c.Node.Path
	}
	if c.Server.Addr == "" {
		c.Server.Addr = fmt.Sprintf("%s:%d", c.Node.Host, c.Node.Port)
	}
	if c.Cluster.NodeID == "" {
		c.Cluster.NodeID = c.Node.NodeID
	}
	if c.Cluster.Topic == "" {
		c.Cluster.Topic = DefaultClusterConfig().Topic
	}
	if c.Cluster.Transport.Topic == "" {
		c.Cluster.Transport.Topic = c.Cluster.Topic
	}
	if c.Cluster.Transport.Type == "" {
		c.Cluster.Transport.Type = "memory"
	}
	if c.Cluster.Transport.Options == nil {
		c.Cluster.Transport.Options = map[string]interface{}{}
	}
	if c.Cluster.Transport.Messaging.Options == nil {
		c.Cluster.Transport.Messaging.Options = map[string]interface{}{}
	}
}

func (c ConfigFile) Validate() error {
	if !c.Node.Enabled {
		return nil
	}
	if c.Node.Port <= 0 {
		return fmt.Errorf("websocket node port must be greater than 0")
	}
	if c.Node.Path == "" || !strings.HasPrefix(c.Node.Path, "/") {
		return fmt.Errorf("websocket node path must start with /")
	}
	if c.Cluster.Enabled {
		transportType := strings.ToLower(strings.TrimSpace(c.Cluster.Transport.Type))
		switch transportType {
		case "memory", "messaging", "rabbitmq", "kafka":
		default:
			return fmt.Errorf("unsupported websocket cluster transport type: %s", c.Cluster.Transport.Type)
		}
		if c.Cluster.Topic == "" {
			return fmt.Errorf("websocket cluster topic is required when cluster is enabled")
		}
	}
	return nil
}
