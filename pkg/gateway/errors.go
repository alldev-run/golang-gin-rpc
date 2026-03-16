package gateway

import "errors"

// Gateway errors
var (
	ErrGatewayAlreadyStarted = errors.New("gateway is already started")
	ErrGatewayNotStarted     = errors.New("gateway is not started")
	ErrRouteNotFound        = errors.New("route not found")
	ErrServiceUnavailable   = errors.New("service unavailable")
	ErrInvalidConfig        = errors.New("invalid configuration")
	ErrDiscoveryFailed      = errors.New("service discovery failed")
	ErrLoadBalancerFailed   = errors.New("load balancer failed")
	ErrRateLimitExceeded    = errors.New("rate limit exceeded")
	ErrTimeout              = errors.New("request timeout")
	ErrTooManyRetries       = errors.New("too many retries")
)
