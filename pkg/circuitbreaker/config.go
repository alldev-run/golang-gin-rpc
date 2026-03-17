package circuitbreaker

import "time"

// Config holds circuit breaker configuration
type Config struct {
	// MaxRequests is the maximum number of requests allowed in half-open state
	MaxRequests uint32 `yaml:"max_requests" json:"max_requests"`
	
	// Interval is the time to collect metrics in closed state
	Interval time.Duration `yaml:"interval" json:"interval"`
	
	// Timeout is the time to wait in open state
	Timeout time.Duration `yaml:"timeout" json:"timeout"`
	
	// ReadyToTrip decides whether to trip the circuit breaker based on metrics
	ReadyToTrip func(counts Counts) bool `yaml:"-" json:"-"`
	
	// OnStateChange is called when the circuit breaker state changes
	OnStateChange func(name string, from State, to State) `yaml:"-" json:"-"`
	
	// IsSuccessful is called to determine if a request succeeded
	IsSuccessful func(err error) bool `yaml:"-" json:"-"`
}

// DefaultConfig returns default circuit breaker configuration
func DefaultConfig() Config {
	return Config{
		MaxRequests: 1,
		Interval:    time.Minute,
		Timeout:     time.Minute * 2,
		ReadyToTrip: DefaultReadyToTrip,
		IsSuccessful: DefaultIsSuccessful,
	}
}

// DefaultReadyToTrip is the default ready-to-trip function
func DefaultReadyToTrip(counts Counts) bool {
	return counts.ConsecutiveFailures > 5
}

// DefaultIsSuccessful is the default successful function
func DefaultIsSuccessful(err error) bool {
	return err == nil
}

// Validate validates the configuration
func (c Config) Validate() error {
	if c.MaxRequests == 0 {
		c.MaxRequests = 1
	}
	if c.Interval == 0 {
		c.Interval = time.Minute
	}
	if c.Timeout == 0 {
		c.Timeout = time.Minute * 2
	}
	if c.ReadyToTrip == nil {
		c.ReadyToTrip = DefaultReadyToTrip
	}
	if c.IsSuccessful == nil {
		c.IsSuccessful = DefaultIsSuccessful
	}
	return nil
}
