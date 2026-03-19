package push

import (
	"fmt"
)

// ProviderType represents the type of push notification provider
type ProviderType string

const (
	// ProviderTypeFCM is Google Firebase Cloud Messaging
	ProviderTypeFCM ProviderType = "fcm"
	// ProviderTypeAPNs is Apple Push Notification service
	ProviderTypeAPNs ProviderType = "apns"
)

// Platform represents the target device platform
type Platform string

const (
	// PlatformAndroid for Android devices
	PlatformAndroid Platform = "android"
	// PlatformIOS for iOS devices
	PlatformIOS Platform = "ios"
)

// Priority represents the notification priority
type Priority string

const (
	// PriorityHigh for high priority notifications
	PriorityHigh Priority = "high"
	// PriorityNormal for normal priority notifications
	PriorityNormal Priority = "normal"
)

// IsValid checks if the provider type is supported
func (pt ProviderType) IsValid() bool {
	supportedTypes := []ProviderType{
		ProviderTypeFCM,
		ProviderTypeAPNs,
	}

	for _, supportedType := range supportedTypes {
		if pt == supportedType {
			return true
		}
	}
	return false
}

// String returns the string representation of ProviderType
func (pt ProviderType) String() string {
	return string(pt)
}

// DisplayName returns a human-readable display name
func (pt ProviderType) DisplayName() string {
	switch pt {
	case ProviderTypeFCM:
		return "Firebase Cloud Messaging (FCM)"
	case ProviderTypeAPNs:
		return "Apple Push Notification Service (APNs)"
	default:
		return "Unknown"
	}
}

// DefaultEndpoint returns the default API endpoint for the provider
func (pt ProviderType) DefaultEndpoint() string {
	switch pt {
	case ProviderTypeFCM:
		return "https://fcm.googleapis.com/v1"
	case ProviderTypeAPNs:
		return "https://api.push.apple.com"
	default:
		return ""
	}
}

// SandboxEndpoint returns the sandbox/development API endpoint
func (pt ProviderType) SandboxEndpoint() string {
	switch pt {
	case ProviderTypeFCM:
		return "https://fcm.googleapis.com/v1" // FCM uses same endpoint
	case ProviderTypeAPNs:
		return "https://api.sandbox.push.apple.com"
	default:
		return ""
	}
}

// RequiresAuthentication returns true if the provider requires authentication
func (pt ProviderType) RequiresAuthentication() bool {
	return true
}

// SupportsTopic returns true if the provider supports topic-based messaging
func (pt ProviderType) SupportsTopic() bool {
	switch pt {
	case ProviderTypeFCM:
		return true
	case ProviderTypeAPNs:
		return true
	default:
		return false
	}
}

// SupportsMulticast returns true if the provider supports multicast
func (pt ProviderType) SupportsMulticast() bool {
	switch pt {
	case ProviderTypeFCM:
		return true
	case ProviderTypeAPNs:
		return true // APNs supports multicast via HTTP/2
	default:
		return false
	}
}

// MaxBatchSize returns the maximum number of tokens per batch request
func (pt ProviderType) MaxBatchSize() int {
	switch pt {
	case ProviderTypeFCM:
		return 500
	case ProviderTypeAPNs:
		return 100
	default:
		return 0
	}
}

// GetSupportedProviders returns all supported provider types
func GetSupportedProviders() []ProviderType {
	return []ProviderType{
		ProviderTypeFCM,
		ProviderTypeAPNs,
	}
}

// ParseProviderType parses a string into ProviderType
func ParseProviderType(s string) (ProviderType, error) {
	switch s {
	case "fcm", "firebase", "gcm":
		return ProviderTypeFCM, nil
	case "apns", "apple", "ios":
		return ProviderTypeAPNs, nil
	default:
		return "", fmt.Errorf("unsupported push provider type: %s", s)
	}
}
