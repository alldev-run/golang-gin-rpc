package push

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/pkcs12"
)

const (
	apnsProductionEndpoint  = "https://api.push.apple.com"
	apnsDevelopmentEndpoint = "https://api.sandbox.push.apple.com"
	apnsPort                = 443
	maxAPNsTokens           = 100
	defaultAPNsTimeout      = 30 * time.Second
	jwtTokenTTL             = 50 * time.Minute // APNs tokens expire after 1 hour, refresh at 50 min
)

// APNsClient is the Apple Push Notification service client
type APNsClient struct {
	config      Config
	httpClient  *http.Client
	endpoint    string
	jwtToken    string
	tokenExpiry time.Time
	keyID       string
	teamID      string
	bundleID    string
	privateKey  *ecdsa.PrivateKey
}

// APNsTokenAuth holds token-based authentication info
type APNsTokenAuth struct {
	AuthKey    *ecdsa.PrivateKey
	KeyID      string
	TeamID     string
	BundleID   string
	Token      string
	Expiry     time.Time
}

// NewAPNsClient creates a new APNs client
func NewAPNsClient(config Config) (*APNsClient, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid APNs config: %w", err)
	}

	client := &APNsClient{
		config:   config,
		keyID:    config.APNs.KeyID,
		teamID:   config.APNs.TeamID,
		bundleID: config.APNs.BundleID,
	}

	// Set endpoint based on sandbox flag
	if config.APNs.UseSandbox {
		client.endpoint = apnsDevelopmentEndpoint
	} else {
		client.endpoint = apnsProductionEndpoint
	}

	timeout := config.Timeout
	if timeout == 0 {
		timeout = defaultAPNsTimeout
	}

	if config.APNs.UseTokenAuth {
		// Setup token-based authentication
		if err := client.setupTokenAuth(); err != nil {
			return nil, fmt.Errorf("failed to setup APNs token auth: %w", err)
		}
	} else {
		// Setup certificate-based authentication
		if err := client.setupCertAuth(); err != nil {
			return nil, fmt.Errorf("failed to setup APNs cert auth: %w", err)
		}
	}

	client.httpClient = &http.Client{
		Timeout: timeout,
	}

	return client, nil
}

// setupTokenAuth initializes JWT token-based authentication
func (c *APNsClient) setupTokenAuth() error {
	var keyData []byte

	if c.config.APNs.AuthKey != "" {
		keyData = []byte(c.config.APNs.AuthKey)
	} else if c.config.APNs.AuthKeyPath != "" {
		// In production, read from file
		return fmt.Errorf("auth key path not implemented, please use AuthKey content")
	}

	if len(keyData) == 0 {
		return fmt.Errorf("no auth key provided")
	}

	privateKey, err := c.parsePKCS8Key(keyData)
	if err != nil {
		return fmt.Errorf("failed to parse auth key: %w", err)
	}

	c.privateKey = privateKey
	return nil
}

// parsePKCS8Key parses a P8 format private key
func (c *APNsClient) parsePKCS8Key(keyData []byte) (*ecdsa.PrivateKey, error) {
	// Try PEM format first
	block, _ := pem.Decode(keyData)
	if block != nil {
		keyData = block.Bytes
	}

	key, err := x509.ParsePKCS8PrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PKCS8 private key: %w", err)
	}

	ecKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("auth key is not an ECDSA key")
	}

	return ecKey, nil
}

// setupCertAuth initializes certificate-based authentication
func (c *APNsClient) setupCertAuth() error {
	if c.config.APNs.CertPath == "" {
		return fmt.Errorf("certificate path is required for certificate auth")
	}

	// In production, load certificate from file
	// For now, we support token auth which is recommended by Apple
	return fmt.Errorf("certificate auth not fully implemented, please use token auth")
}

// getJWTToken returns a valid JWT token, generating a new one if needed
func (c *APNsClient) getJWTToken() (string, error) {
	if c.jwtToken != "" && time.Now().Before(c.tokenExpiry) {
		return c.jwtToken, nil
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": c.teamID,
		"iat": time.Now().Unix(),
	})
	token.Header["kid"] = c.keyID

	tokenString, err := token.SignedString(c.privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT token: %w", err)
	}

	c.jwtToken = tokenString
	c.tokenExpiry = time.Now().Add(jwtTokenTTL)

	return tokenString, nil
}

// Send sends a notification to a single device
func (c *APNsClient) Send(ctx context.Context, notification *Notification) (*Response, error) {
	if err := notification.IsValid(); err != nil {
		return nil, err
	}

	if len(notification.Tokens) == 0 {
		return nil, fmt.Errorf("APNs send requires at least one token")
	}

	token := notification.Tokens[0]
	return c.sendToToken(ctx, notification, token)
}

// sendToToken sends a notification to a specific device token
func (c *APNsClient) sendToToken(ctx context.Context, notification *Notification, token string) (*Response, error) {
	payload := c.buildPayload(notification)

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal APNs payload: %w", err)
	}

	endpoint := fmt.Sprintf("%s/3/device/%s", c.endpoint, token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create APNs request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apns-topic", c.bundleID)
	req.Header.Set("apns-push-type", "alert")
	req.Header.Set("apns-priority", c.getPriority(notification.Priority))

	if notification.TTL > 0 {
		req.Header.Set("apns-expiration", fmt.Sprintf("%d", time.Now().Add(notification.TTL).Unix()))
	}

	// Set authorization for token-based auth
	if c.config.APNs.UseTokenAuth {
		jwtToken, err := c.getJWTToken()
		if err != nil {
			return nil, fmt.Errorf("failed to get JWT token: %w", err)
		}
		req.Header.Set("authorization", fmt.Sprintf("bearer %s", jwtToken))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("APNs request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read APNs response: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		// Get apns-id from response header
		messageID := resp.Header.Get("apns-id")

		return &Response{
			Success:   true,
			MessageID: messageID,
			Token:     token,
			Provider:  ProviderTypeAPNs,
			SentAt:    time.Now(),
		}, nil
	}

	return c.parseErrorResponse(body, resp.StatusCode, token)
}

// SendMulticast sends a notification to multiple devices using HTTP/2 multiplexing
func (c *APNsClient) SendMulticast(ctx context.Context, notification *Notification, tokens []string) (*BatchResponse, error) {
	if len(tokens) == 0 {
		return nil, fmt.Errorf("no tokens provided for multicast")
	}

	if len(tokens) > maxAPNsTokens {
		return nil, fmt.Errorf("too many tokens: %d (max %d)", len(tokens), maxAPNsTokens)
	}

	responses := make([]*Response, 0, len(tokens))
	successCount := 0
	failureCount := 0

	// APNs doesn't have a native multicast API, we send individual requests
	// HTTP/2 allows multiplexing over a single connection
	for _, token := range tokens {
		resp, err := c.sendToToken(ctx, notification, token)
		if err != nil {
			failureCount++
			responses = append(responses, &Response{
				Success:  false,
				Token:    token,
				Error:    &Error{Code: "REQUEST_FAILED", Message: err.Error()},
				Provider: ProviderTypeAPNs,
				SentAt:   time.Now(),
			})
			continue
		}

		if resp.Success {
			successCount++
		} else {
			failureCount++
		}
		responses = append(responses, resp)
	}

	return &BatchResponse{
		SuccessCount: successCount,
		FailureCount: failureCount,
		Responses:    responses,
		Provider:     ProviderTypeAPNs,
		SentAt:       time.Now(),
	}, nil
}

// SendToTopic sends a notification to a topic
func (c *APNsClient) SendToTopic(ctx context.Context, notification *Notification, topic string) (*Response, error) {
	// APNs doesn't support topic-based messaging directly
	// This requires a separate topic subscription service
	return nil, fmt.Errorf("APNs topic messaging requires separate topic management service")
}

// SubscribeToTopic subscribes tokens to a topic
func (c *APNsClient) SubscribeToTopic(ctx context.Context, tokens []string, topic string) error {
	// APNs doesn't have native topic support
	// Topics are managed through your own backend service
	return fmt.Errorf("APNs topic subscription requires separate topic management service")
}

// UnsubscribeFromTopic unsubscribes tokens from a topic
func (c *APNsClient) UnsubscribeFromTopic(ctx context.Context, tokens []string, topic string) error {
	// APNs doesn't have native topic support
	return fmt.Errorf("APNs topic unsubscription requires separate topic management service")
}

// Close closes the client
func (c *APNsClient) Close() error {
	// Nothing to close for HTTP client
	return nil
}

// Provider returns the provider type
func (c *APNsClient) Provider() ProviderType {
	return ProviderTypeAPNs
}

// IsHealthy checks if the client is healthy
func (c *APNsClient) IsHealthy(ctx context.Context) error {
	if c.config.APNs.UseTokenAuth {
		_, err := c.getJWTToken()
		if err != nil {
			return fmt.Errorf("failed to generate JWT token: %w", err)
		}
	}
	return nil
}

// buildPayload builds the APNs payload
func (c *APNsClient) buildPayload(n *Notification) map[string]interface{} {
	aps := map[string]interface{}{
		"alert": map[string]interface{}{
			"title": n.Title,
			"body":  n.Body,
		},
	}

	if n.Sound != "" {
		aps["sound"] = n.Sound
	}

	if n.Badge > 0 {
		aps["badge"] = n.Badge
	}

	if n.Category != "" {
		aps["category"] = n.Category
	}

	payload := map[string]interface{}{
		"aps": aps,
	}

	// Add custom data
	for key, value := range n.Data {
		payload[key] = value
	}

	return payload
}

// getPriority returns the APNs priority value
func (c *APNsClient) getPriority(priority Priority) string {
	switch priority {
	case PriorityHigh:
		return "10"
	case PriorityNormal:
		return "5"
	default:
		return "10"
	}
}

// parseErrorResponse parses an APNs error response
func (c *APNsClient) parseErrorResponse(body []byte, statusCode int, token string) (*Response, error) {
	var errorResp struct {
		Reason    string `json:"reason"`
		Timestamp int64  `json:"timestamp"`
	}

	json.Unmarshal(body, &errorResp)

	err := &Error{
		Code:    errorResp.Reason,
		Message: errorResp.Reason,
	}

	// Map APNs error reasons
	switch errorResp.Reason {
	case "BadDeviceToken":
		err.Message = "The device token is invalid"
		err.InvalidToken = true
		err.IsRecoverable = false
	case "Unregistered":
		err.Message = "The device token is no longer valid"
		err.InvalidToken = true
		err.IsRecoverable = false
	case "BadTopic":
		err.Message = "The topic is invalid"
		err.IsRecoverable = false
	case "TopicDisallowed":
		err.Message = "Push notifications for the specified topic are not allowed"
		err.IsRecoverable = false
	case "PayloadEmpty":
		err.Message = "The message payload was empty"
		err.IsRecoverable = false
	case "PayloadTooLarge":
		err.Message = "The message payload is too large"
		err.IsRecoverable = false
	case "InvalidProviderToken":
		err.Message = "The provider token is invalid"
		err.IsRecoverable = false
	case "ExpiredProviderToken":
		err.Message = "The provider token has expired"
		err.IsRecoverable = true
	case "BadCertificate":
		err.Message = "The certificate is invalid"
		err.IsRecoverable = false
	case "BadCertificateEnvironment":
		err.Message = "The certificate environment is incorrect"
		err.IsRecoverable = false
	case "ExpiredToken":
		err.Message = "The device token has expired"
		err.InvalidToken = true
		err.IsRecoverable = false
	case "TooManyRequests":
		err.Message = "Too many requests were made"
		err.IsRecoverable = true
	case "InternalServerError":
		err.Message = "An internal server error occurred"
		err.IsRecoverable = true
	case "ServiceUnavailable":
		err.Message = "The service is unavailable"
		err.IsRecoverable = true
	case "Shutdown":
		err.Message = "The server is shutting down"
		err.IsRecoverable = true
	default:
		if statusCode >= 500 {
			err.IsRecoverable = true
		}
	}

	return &Response{
		Success:  false,
		Token:    token,
		Error:    err,
		Provider: ProviderTypeAPNs,
		SentAt:   time.Now(),
	}, nil
}

// Helper function to parse PKCS12 certificate (for certificate auth)
func parsePKCS12(certData []byte, password string) (*tls.Certificate, error) {
	blocks, err := pkcs12.ToPEM(certData, password)
	if err != nil {
		return nil, fmt.Errorf("failed to decode PKCS12: %w", err)
	}

	var certPEM, keyPEM []byte
	for _, block := range blocks {
		if block.Type == "CERTIFICATE" {
			certPEM = append(certPEM, pem.EncodeToMemory(block)...)
		} else if block.Type == "PRIVATE KEY" || strings.Contains(block.Type, "PRIVATE KEY") {
			keyPEM = append(keyPEM, pem.EncodeToMemory(block)...)
		}
	}

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to load X509 key pair: %w", err)
	}

	return &cert, nil
}
