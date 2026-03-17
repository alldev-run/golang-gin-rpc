package ratelimiter

import "time"

// Strategy represents the rate limiting strategy
type Strategy string

const (
	// StrategyTokenBucket uses token bucket algorithm
	StrategyTokenBucket Strategy = "token_bucket"
	// StrategySlidingWindow uses sliding window algorithm
	StrategySlidingWindow Strategy = "sliding_window"
	// StrategyFixedWindow uses fixed window algorithm
	StrategyFixedWindow Strategy = "fixed_window"
)

// Config holds rate limiter configuration
type Config struct {
	// Strategy is the rate limiting strategy
	Strategy Strategy `yaml:"strategy" json:"strategy"`
	
	// Rate is the number of requests allowed per second
	Rate float64 `yaml:"rate" json:"rate"`
	
	// Burst is the maximum burst size
	Burst int `yaml:"burst" json:"burst"`
	
	// WindowSize is the window size for window-based algorithms
	WindowSize time.Duration `yaml:"window_size" json:"window_size"`
	
	// CleanupInterval is the interval for cleaning up old data
	CleanupInterval time.Duration `yaml:"cleanup_interval" json:"cleanup_interval"`
	
	// MaxKeys is the maximum number of keys to track
	MaxKeys int `yaml:"max_keys" json:"max_keys"`
	
	// Enabled indicates if rate limiting is enabled
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// DefaultConfig returns default rate limiter configuration
func DefaultConfig() Config {
	return Config{
		Strategy:         StrategyTokenBucket,
		Rate:             100,              // 100 requests per second
		Burst:            10,               // Allow burst of 10 requests
		WindowSize:       time.Minute,      // 1 minute window for window-based strategies
		CleanupInterval:  5 * time.Minute,  // Clean up old data every 5 minutes
		MaxKeys:          10000,            // Track up to 10,000 different keys
		Enabled:          true,
	}
}

// HighTrafficConfig returns configuration for high traffic scenarios
func HighTrafficConfig() Config {
	return Config{
		Strategy:         StrategyTokenBucket,
		Rate:             1000,             // 1000 requests per second
		Burst:            100,              // Allow burst of 100 requests
		WindowSize:       time.Minute,
		CleanupInterval:  time.Minute,
		MaxKeys:          100000,           // Track up to 100,000 different keys
		Enabled:          true,
	}
}

// LowTrafficConfig returns configuration for low traffic scenarios
func LowTrafficConfig() Config {
	return Config{
		Strategy:         StrategySlidingWindow,
		Rate:             10,               // 10 requests per second
		Burst:            5,                // Allow burst of 5 requests
		WindowSize:       time.Minute,
		CleanupInterval:  10 * time.Minute,
		MaxKeys:          1000,             // Track up to 1,000 different keys
		Enabled:          true,
	}
}

// APIConfig returns configuration for API endpoints
func APIConfig() Config {
	return Config{
		Strategy:         StrategyTokenBucket,
		Rate:             60,               // 60 requests per minute (1 per second)
		Burst:            10,               // Allow burst of 10 requests
		WindowSize:       time.Minute,
		CleanupInterval:  5 * time.Minute,
		MaxKeys:          10000,            // Track up to 10,000 different clients
		Enabled:          true,
	}
}

// Validate validates the configuration
func (c Config) Validate() error {
	if c.Strategy == "" {
		c.Strategy = StrategyTokenBucket
	}
	if c.Rate <= 0 {
		c.Rate = 100
	}
	if c.Burst <= 0 {
		c.Burst = 10
	}
	if c.WindowSize == 0 {
		c.WindowSize = time.Minute
	}
	if c.CleanupInterval == 0 {
		c.CleanupInterval = 5 * time.Minute
	}
	if c.MaxKeys <= 0 {
		c.MaxKeys = 10000
	}
	return nil
}
