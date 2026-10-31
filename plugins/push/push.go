package push

import (
	"context"
	"fmt"
	"time"
)

// Client is the interface for push notification clients
type Client interface {
	// Send sends a notification to a single device
	Send(ctx context.Context, notification *Notification) (*Response, error)

	// SendMulticast sends a notification to multiple devices
	SendMulticast(ctx context.Context, notification *Notification, tokens []string) (*BatchResponse, error)

	// SendToTopic sends a notification to a topic/subscribed devices
	SendToTopic(ctx context.Context, notification *Notification, topic string) (*Response, error)

	// SubscribeToTopic subscribes tokens to a topic
	SubscribeToTopic(ctx context.Context, tokens []string, topic string) error

	// UnsubscribeFromTopic unsubscribes tokens from a topic
	UnsubscribeFromTopic(ctx context.Context, tokens []string, topic string) error

	// Close closes the client connection
	Close() error

	// Provider returns the provider type
	Provider() ProviderType

	// IsHealthy checks if the client is healthy
	IsHealthy(ctx context.Context) error
}

// Notification represents a push notification message
type Notification struct {
	// Tokens are the device registration tokens (FCM) or device tokens (APNs)
	// Note: Use either Tokens or Topic, not both
	Tokens []string `json:"tokens,omitempty"`

	// Topic is the topic name for topic-based messaging
	Topic string `json:"topic,omitempty"`

	// Platform is the target platform (Android or iOS)
	Platform Platform `json:"platform"`

	// Title is the notification title
	Title string `json:"title"`

	// Body is the notification body/message
	Body string `json:"body"`

	// Image is the URL to an image for rich notifications
	Image string `json:"image,omitempty"`

	// Icon is the notification icon (Android only)
	Icon string `json:"icon,omitempty"`

	// Sound is the sound to play (default: "default")
	Sound string `json:"sound,omitempty"`

	// Badge is the badge number to display (iOS only)
	Badge int `json:"badge,omitempty"`

	// Priority is the notification priority (high or normal)
	Priority Priority `json:"priority,omitempty"`

	// TTL is the time-to-live duration (how long to keep trying to deliver)
	TTL time.Duration `json:"ttl,omitempty"`

	// CollapseKey is a key to collapse/group similar notifications (Android only)
	CollapseKey string `json:"collapse_key,omitempty"`

	// ChannelID is the Android notification channel ID
	ChannelID string `json:"channel_id,omitempty"`

	// Category is the notification category (iOS only)
	Category string `json:"category,omitempty"`

	// Data is custom key-value data to send with the notification
	Data map[string]string `json:"data,omitempty"`

	// Action is the action to perform when user taps the notification
	Action string `json:"action,omitempty"`

	// ClickAction is the action when user clicks the notification (FCM)
	ClickAction string `json:"click_action,omitempty"`

	// DeepLink is a deep link URL to open when notification is tapped
	DeepLink string `json:"deep_link,omitempty"`
}

// Response represents the response from a single push notification request
type Response struct {
	// Success indicates if the request was successful
	Success bool `json:"success"`

	// MessageID is the unique message ID from the provider
	MessageID string `json:"message_id,omitempty"`

	// Token is the device token that was used
	Token string `json:"token,omitempty"`

	// Error contains error information if the request failed
	Error *Error `json:"error,omitempty"`

	// Provider is the provider type
	Provider ProviderType `json:"provider"`

	// SentAt is the timestamp when the notification was sent
	SentAt time.Time `json:"sent_at"`
}

// BatchResponse represents the response from a multicast push notification request
type BatchResponse struct {
	// SuccessCount is the number of successful sends
	SuccessCount int `json:"success_count"`

	// FailureCount is the number of failed sends
	FailureCount int `json:"failure_count"`

	// Responses contains individual responses for each token
	Responses []*Response `json:"responses"`

	// Provider is the provider type
	Provider ProviderType `json:"provider"`

	// SentAt is the timestamp when the batch was sent
	SentAt time.Time `json:"sent_at"`
}

// Error represents an error from the push notification provider
type Error struct {
	// Code is the error code
	Code string `json:"code"`

	// Message is the human-readable error message
	Message string `json:"message"`

	// IsRecoverable indicates if the error is recoverable (can be retried)
	IsRecoverable bool `json:"is_recoverable"`

	// InvalidToken indicates if the token is invalid and should be removed
	InvalidToken bool `json:"invalid_token"`
}

// Error implements the error interface
func (e *Error) Error() string {
	return e.Message
}

// NewNotification creates a basic notification
func NewNotification(title, body string, platform Platform) *Notification {
	return &Notification{
		Title:    title,
		Body:     body,
		Platform: platform,
		Priority: PriorityNormal,
		Sound:    "default",
	}
}

// WithTokens sets the target tokens
func (n *Notification) WithTokens(tokens ...string) *Notification {
	n.Tokens = tokens
	return n
}

// WithTopic sets the target topic
func (n *Notification) WithTopic(topic string) *Notification {
	n.Topic = topic
	return n
}

// WithData adds custom data
func (n *Notification) WithData(key, value string) *Notification {
	if n.Data == nil {
		n.Data = make(map[string]string)
	}
	n.Data[key] = value
	return n
}

// WithHighPriority sets high priority
func (n *Notification) WithHighPriority() *Notification {
	n.Priority = PriorityHigh
	return n
}

// WithBadge sets the badge count (iOS)
func (n *Notification) WithBadge(count int) *Notification {
	n.Badge = count
	return n
}

// WithSound sets the sound
func (n *Notification) WithSound(sound string) *Notification {
	n.Sound = sound
	return n
}

// WithImage sets the image URL
func (n *Notification) WithImage(imageURL string) *Notification {
	n.Image = imageURL
	return n
}

// WithDeepLink sets the deep link
func (n *Notification) WithDeepLink(link string) *Notification {
	n.DeepLink = link
	return n
}

// WithTTL sets the time-to-live
func (n *Notification) WithTTL(ttl time.Duration) *Notification {
	n.TTL = ttl
	return n
}

// IsValid checks if the notification is valid
func (n *Notification) IsValid() error {
	if n.Title == "" {
		return fmt.Errorf("notification title is required")
	}
	if n.Body == "" {
		return fmt.Errorf("notification body is required")
	}
	if n.Platform != PlatformAndroid && n.Platform != PlatformIOS {
		return fmt.Errorf("invalid platform: %s", n.Platform)
	}
	if len(n.Tokens) == 0 && n.Topic == "" {
		return fmt.Errorf("either tokens or topic must be specified")
	}
	return nil
}
