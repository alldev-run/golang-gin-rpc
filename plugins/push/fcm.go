package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	fcmEndpoint        = "https://fcm.googleapis.com/v1/projects/%s/messages:send"
	fcmBatchEndpoint   = "https://fcm.googleapis.com/batch"
	fcmTopicEndpoint   = "https://iid.googleapis.com/iid/v1:batchAdd"
	fcmUnsubEndpoint   = "https://iid.googleapis.com/iid/v1:batchRemove"
	fcmLegacyEndpoint  = "https://fcm.googleapis.com/fcm/send"
	maxFCMTokens       = 500
	defaultFCMTimeout  = 30 * time.Second
)

// FCMClient is the Firebase Cloud Messaging client
type FCMClient struct {
	config     Config
	httpClient *http.Client
	tokenSource oauth2.TokenSource
	projectID  string
}

// NewFCMClient creates a new FCM client
func NewFCMClient(config Config) (*FCMClient, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid FCM config: %w", err)
	}

	client := &FCMClient{
		config:    config,
		projectID: config.FCM.GetProjectID(),
	}

	timeout := config.Timeout
	if timeout == 0 {
		timeout = defaultFCMTimeout
	}

	if config.FCM.UseLegacyAPI {
		client.httpClient = &http.Client{
			Timeout: timeout,
		}
	} else {
		// Setup OAuth2 token source for HTTP v1 API
		if err := client.setupTokenSource(); err != nil {
			return nil, fmt.Errorf("failed to setup FCM token source: %w", err)
		}
	}

	return client, nil
}

// setupTokenSource initializes the OAuth2 token source
func (c *FCMClient) setupTokenSource() error {
	var credentials []byte

	if c.config.FCM.ServiceAccountKey != "" {
		credentials = []byte(c.config.FCM.ServiceAccountKey)
	} else if c.config.FCM.ServiceAccountKeyPath != "" {
		// In a real implementation, read from file
		// For now, we'll expect the key content to be provided
		return fmt.Errorf("service account key path not implemented, please use ServiceAccountKey")
	}

	if len(credentials) == 0 {
		return fmt.Errorf("no service account credentials provided")
	}

	// Parse credentials
	var creds ServiceAccountCredentials
	if err := json.Unmarshal(credentials, &creds); err != nil {
		return fmt.Errorf("failed to parse service account credentials: %w", err)
	}
	c.config.FCM.Credentials = creds

	if c.projectID == "" {
		c.projectID = creds.ProjectID
	}

	config, err := google.JWTConfigFromJSON(credentials, "https://www.googleapis.com/auth/firebase.messaging")
	if err != nil {
		return fmt.Errorf("failed to create JWT config: %w", err)
	}

	c.tokenSource = config.TokenSource(context.Background())
	c.httpClient = oauth2.NewClient(context.Background(), c.tokenSource)
	c.httpClient.Timeout = c.config.Timeout

	return nil
}

// Send sends a notification to a single device
func (c *FCMClient) Send(ctx context.Context, notification *Notification) (*Response, error) {
	if err := notification.IsValid(); err != nil {
		return nil, err
	}

	if len(notification.Tokens) == 0 {
		return nil, fmt.Errorf("FCM send requires at least one token")
	}

	token := notification.Tokens[0]

	if c.config.FCM.UseLegacyAPI {
		return c.sendLegacy(ctx, notification, token)
	}

	return c.sendV1(ctx, notification, token)
}

// sendV1 sends using FCM HTTP v1 API
func (c *FCMClient) sendV1(ctx context.Context, notification *Notification, token string) (*Response, error) {
	message := c.buildV1Message(notification, token, "")

	payload := map[string]interface{}{
		"message": message,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal FCM message: %w", err)
	}

	endpoint := fmt.Sprintf(fcmEndpoint, c.projectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create FCM request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("FCM request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read FCM response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return c.parseErrorResponse(body, resp.StatusCode, token)
	}

	var result struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse FCM response: %w", err)
	}

	return &Response{
		Success:   true,
		MessageID:  result.Name,
		Token:      token,
		Provider:   ProviderTypeFCM,
		SentAt:     time.Now(),
	}, nil
}

// sendLegacy sends using FCM legacy HTTP API
func (c *FCMClient) sendLegacy(ctx context.Context, notification *Notification, token string) (*Response, error) {
	message := c.buildLegacyMessage(notification, token)

	jsonData, err := json.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal legacy FCM message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fcmLegacyEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create legacy FCM request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "key="+c.config.FCM.LegacyServerKey)

	httpClient := &http.Client{Timeout: c.config.Timeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("legacy FCM request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read legacy FCM response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return c.parseLegacyErrorResponse(body, resp.StatusCode, token)
	}

	var result struct {
		MulticastID  int64 `json:"multicast_id"`
		Success      int   `json:"success"`
		Failure      int   `json:"failure"`
		CanonicalIDs int   `json:"canonical_ids"`
		Results      []struct {
			MessageID      string `json:"message_id"`
			Error          string `json:"error"`
			RegistrationID string `json:"registration_id"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse legacy FCM response: %w", err)
	}

	if result.Success > 0 && len(result.Results) > 0 {
		return &Response{
			Success:   true,
			MessageID: result.Results[0].MessageID,
			Token:     token,
			Provider:  ProviderTypeFCM,
			SentAt:    time.Now(),
		}, nil
	}

	if result.Failure > 0 && len(result.Results) > 0 {
		return &Response{
			Success: false,
			Token:   token,
			Error:   c.mapLegacyError(result.Results[0].Error),
			Provider: ProviderTypeFCM,
			SentAt:  time.Now(),
		}, nil
	}

	return &Response{
		Success: false,
		Token:   token,
		Error: &Error{
			Code:    "UNKNOWN_ERROR",
			Message: "Unknown FCM error",
		},
		Provider: ProviderTypeFCM,
		SentAt:   time.Now(),
	}, nil
}

// SendMulticast sends a notification to multiple devices
func (c *FCMClient) SendMulticast(ctx context.Context, notification *Notification, tokens []string) (*BatchResponse, error) {
	if len(tokens) == 0 {
		return nil, fmt.Errorf("no tokens provided for multicast")
	}

	if len(tokens) > maxFCMTokens {
		return nil, fmt.Errorf("too many tokens: %d (max %d)", len(tokens), maxFCMTokens)
	}

	notification.Tokens = tokens

	if c.config.FCM.UseLegacyAPI {
		return c.sendMulticastLegacy(ctx, notification, tokens)
	}

	return c.sendMulticastV1(ctx, notification, tokens)
}

// sendMulticastV1 sends multicast using FCM HTTP v1 API (batch)
func (c *FCMClient) sendMulticastV1(ctx context.Context, notification *Notification, tokens []string) (*BatchResponse, error) {
	responses := make([]*Response, 0, len(tokens))
	successCount := 0
	failureCount := 0

	// FCM v1 doesn't have native multicast, we send individual requests
	// In production, you'd want to use batch HTTP requests
	for _, token := range tokens {
		resp, err := c.sendV1(ctx, notification, token)
		if err != nil {
			failureCount++
			responses = append(responses, &Response{
				Success:  false,
				Token:    token,
				Error:    &Error{Code: "REQUEST_FAILED", Message: err.Error()},
				Provider: ProviderTypeFCM,
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
		Provider:     ProviderTypeFCM,
		SentAt:       time.Now(),
	}, nil
}

// sendMulticastLegacy sends multicast using FCM legacy API
func (c *FCMClient) sendMulticastLegacy(ctx context.Context, notification *Notification, tokens []string) (*BatchResponse, error) {
	message := c.buildLegacyMessage(notification, "")
	message["registration_ids"] = tokens

	jsonData, err := json.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal legacy multicast message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fcmLegacyEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create legacy multicast request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "key="+c.config.FCM.LegacyServerKey)

	httpClient := &http.Client{Timeout: c.config.Timeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("legacy multicast request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read legacy multicast response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("FCM legacy multicast failed: %s", string(body))
	}

	var result struct {
		MulticastID  int64 `json:"multicast_id"`
		Success      int   `json:"success"`
		Failure      int   `json:"failure"`
		CanonicalIDs int   `json:"canonical_ids"`
		Results      []struct {
			MessageID      string `json:"message_id"`
			Error          string `json:"error"`
			RegistrationID string `json:"registration_id"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse legacy multicast response: %w", err)
	}

	responses := make([]*Response, 0, len(tokens))
	for i, token := range tokens {
		if i < len(result.Results) {
			r := result.Results[i]
			if r.Error == "" {
				responses = append(responses, &Response{
					Success:   true,
					MessageID: r.MessageID,
					Token:     token,
					Provider:  ProviderTypeFCM,
					SentAt:    time.Now(),
				})
			} else {
				responses = append(responses, &Response{
					Success:  false,
					Token:    token,
					Error:    c.mapLegacyError(r.Error),
					Provider: ProviderTypeFCM,
					SentAt:   time.Now(),
				})
			}
		}
	}

	return &BatchResponse{
		SuccessCount: result.Success,
		FailureCount: result.Failure,
		Responses:    responses,
		Provider:     ProviderTypeFCM,
		SentAt:       time.Now(),
	}, nil
}

// SendToTopic sends a notification to a topic
func (c *FCMClient) SendToTopic(ctx context.Context, notification *Notification, topic string) (*Response, error) {
	if topic == "" {
		return nil, fmt.Errorf("topic is required")
	}

	if c.config.FCM.UseLegacyAPI {
		notification.Tokens = []string{"/topics/" + topic}
		return c.sendLegacy(ctx, notification, "/topics/"+topic)
	}

	message := c.buildV1Message(notification, "", topic)

	payload := map[string]interface{}{
		"message": message,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal topic message: %w", err)
	}

	endpoint := fmt.Sprintf(fcmEndpoint, c.projectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create topic request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("topic request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read topic response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return c.parseErrorResponse(body, resp.StatusCode, "")
	}

	var result struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse topic response: %w", err)
	}

	return &Response{
		Success:   true,
		MessageID: result.Name,
		Provider:  ProviderTypeFCM,
		SentAt:    time.Now(),
	}, nil
}

// SubscribeToTopic subscribes tokens to a topic
func (c *FCMClient) SubscribeToTopic(ctx context.Context, tokens []string, topic string) error {
	if c.config.FCM.UseLegacyAPI {
		return fmt.Errorf("topic subscription not supported in legacy API")
	}

	// FCM v1 uses Instance ID API for topic management
	// This requires the legacy server key even with v1 API
	if c.config.FCM.LegacyServerKey == "" {
		return fmt.Errorf("legacy server key required for topic management")
	}

	return c.manageTopic(ctx, tokens, topic, true)
}

// UnsubscribeFromTopic unsubscribes tokens from a topic
func (c *FCMClient) UnsubscribeFromTopic(ctx context.Context, tokens []string, topic string) error {
	if c.config.FCM.UseLegacyAPI {
		return fmt.Errorf("topic unsubscription not supported in legacy API")
	}

	if c.config.FCM.LegacyServerKey == "" {
		return fmt.Errorf("legacy server key required for topic management")
	}

	return c.manageTopic(ctx, tokens, topic, false)
}

// manageTopic handles topic subscription/unsubscription
func (c *FCMClient) manageTopic(ctx context.Context, tokens []string, topic string, subscribe bool) error {
	if len(tokens) == 0 {
		return fmt.Errorf("no tokens provided")
	}
	if topic == "" {
		return fmt.Errorf("no topic provided")
	}
	if len(tokens) > 1000 {
		return fmt.Errorf("too many tokens: max 1000")
	}

	payload := map[string]interface{}{
		"to":                 "/topics/" + topic,
		"registration_tokens": tokens,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal topic request: %w", err)
	}

	endpoint := fcmTopicEndpoint
	if !subscribe {
		endpoint = fcmUnsubEndpoint
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create topic management request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "key="+c.config.FCM.LegacyServerKey)

	httpClient := &http.Client{Timeout: c.config.Timeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("topic management request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read topic management response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("topic management failed: %s", string(body))
	}

	var result struct {
		Results []struct {
			Error string `json:"error"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse topic management response: %w", err)
	}

	for _, r := range result.Results {
		if r.Error != "" {
			return fmt.Errorf("topic management error: %s", r.Error)
		}
	}

	return nil
}

// Close closes the client
func (c *FCMClient) Close() error {
	// Nothing to close for HTTP client
	return nil
}

// Provider returns the provider type
func (c *FCMClient) Provider() ProviderType {
	return ProviderTypeFCM
}

// IsHealthy checks if the client is healthy
func (c *FCMClient) IsHealthy(ctx context.Context) error {
	// Try to get a token to verify credentials
	if !c.config.FCM.UseLegacyAPI && c.tokenSource != nil {
		token, err := c.tokenSource.Token()
		if err != nil {
			return fmt.Errorf("failed to get OAuth token: %w", err)
		}
		if !token.Valid() {
			return fmt.Errorf("invalid OAuth token")
		}
	}
	return nil
}

// buildV1Message builds a FCM v1 message
func (c *FCMClient) buildV1Message(n *Notification, token, topic string) map[string]interface{} {
	message := map[string]interface{}{}

	// Target
	if token != "" {
		message["token"] = token
	} else if topic != "" {
		message["topic"] = topic
	}

	// Android config
	androidConfig := map[string]interface{}{}
	if n.Priority == PriorityHigh {
		androidConfig["priority"] = "high"
	} else {
		androidConfig["priority"] = "normal"
	}
	if n.CollapseKey != "" {
		androidConfig["collapse_key"] = n.CollapseKey
	}
	if n.TTL > 0 {
		androidConfig["ttl"] = n.TTL.String()
	}

	// Android notification
	androidNotification := map[string]interface{}{
		"title": n.Title,
		"body":  n.Body,
	}
	if n.Sound != "" {
		androidNotification["sound"] = n.Sound
	}
	if n.Icon != "" {
		androidNotification["icon"] = n.Icon
	}
	if n.ChannelID != "" {
		androidNotification["channel_id"] = n.ChannelID
	}
	if n.Image != "" {
		androidNotification["image"] = n.Image
	}
	if n.ClickAction != "" {
		androidNotification["click_action"] = n.ClickAction
	}

	androidConfig["notification"] = androidNotification
	message["android"] = androidConfig

	// APNS config (for iOS via FCM)
	apnsConfig := map[string]interface{}{
		"payload": map[string]interface{}{
			"aps": map[string]interface{}{
				"alert": map[string]interface{}{
					"title": n.Title,
					"body":  n.Body,
				},
				"sound": n.Sound,
				"badge": n.Badge,
			},
		},
	}
	if n.Category != "" {
		apnsPayload := apnsConfig["payload"].(map[string]interface{})["aps"].(map[string]interface{})
		apnsPayload["category"] = n.Category
	}
	message["apns"] = apnsConfig

	// Data
	if len(n.Data) > 0 {
		message["data"] = n.Data
	}

	return message
}

// buildLegacyMessage builds a FCM legacy message
func (c *FCMClient) buildLegacyMessage(n *Notification, token string) map[string]interface{} {
	message := map[string]interface{}{
		"notification": map[string]interface{}{
			"title": n.Title,
			"body":  n.Body,
		},
		"data": n.Data,
	}

	if token != "" {
		message["to"] = token
	}

	// Android specific options
	if n.Priority == PriorityHigh {
		message["priority"] = "high"
	}
	if n.CollapseKey != "" {
		message["collapse_key"] = n.CollapseKey
	}
	if n.TTL > 0 {
		message["time_to_live"] = int(n.TTL.Seconds())
	}

	// Add sound to notification
	if n.Sound != "" {
		notification := message["notification"].(map[string]interface{})
		notification["sound"] = n.Sound
	}

	return message
}

// parseErrorResponse parses an error response from FCM v1
func (c *FCMClient) parseErrorResponse(body []byte, statusCode int, token string) (*Response, error) {
	var errorResp struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Status  string `json:"status"`
			Details []struct {
				Type     string `json:"@type"`
				ErrorCode string `json:"errorCode,omitempty"`
			} `json:"details"`
		} `json:"error"`
	}

	json.Unmarshal(body, &errorResp)

	err := &Error{
		Code:    errorResp.Error.Status,
		Message: errorResp.Error.Message,
	}

	// Map specific error codes
	switch errorResp.Error.Status {
	case "UNREGISTERED":
		err.Code = "UNREGISTERED"
		err.InvalidToken = true
		err.IsRecoverable = false
	case "SENDER_ID_MISMATCH":
		err.Code = "SENDER_ID_MISMATCH"
		err.InvalidToken = true
		err.IsRecoverable = false
	case "QUOTA_EXCEEDED":
		err.Code = "QUOTA_EXCEEDED"
		err.IsRecoverable = true
	case "UNAVAILABLE":
		err.Code = "UNAVAILABLE"
		err.IsRecoverable = true
	case "INTERNAL":
		err.Code = "INTERNAL"
		err.IsRecoverable = true
	default:
		err.IsRecoverable = statusCode >= 500
	}

	return &Response{
		Success:  false,
		Token:    token,
		Error:    err,
		Provider: ProviderTypeFCM,
		SentAt:   time.Now(),
	}, nil
}

// parseLegacyErrorResponse parses a legacy FCM error response
func (c *FCMClient) parseLegacyErrorResponse(body []byte, statusCode int, token string) (*Response, error) {
	err := &Error{
		Code:    fmt.Sprintf("HTTP_%d", statusCode),
		Message: string(body),
	}

	if statusCode >= 500 {
		err.IsRecoverable = true
	}

	return &Response{
		Success:  false,
		Token:    token,
		Error:    err,
		Provider: ProviderTypeFCM,
		SentAt:   time.Now(),
	}, nil
}

// mapLegacyError maps legacy FCM error codes
func (c *FCMClient) mapLegacyError(errorCode string) *Error {
	err := &Error{
		Code:    errorCode,
		Message: errorCode,
	}

	switch strings.ToLower(errorCode) {
	case "missing_registration":
		err.Message = "Missing registration token"
		err.InvalidToken = true
		err.IsRecoverable = false
	case "invalid_registration":
		err.Message = "Invalid registration token"
		err.InvalidToken = true
		err.IsRecoverable = false
	case "not_registered":
		err.Message = "Token not registered"
		err.InvalidToken = true
		err.IsRecoverable = false
	case "invalid_package_name":
		err.Message = "Invalid package name"
		err.IsRecoverable = false
	case "mismatch_sender_id":
		err.Message = "Mismatched sender ID"
		err.InvalidToken = true
		err.IsRecoverable = false
	case "invalid_parameters":
		err.Message = "Invalid parameters"
		err.IsRecoverable = false
	case "message_too_big":
		err.Message = "Message too big"
		err.IsRecoverable = false
	case "invalid_data_key":
		err.Message = "Invalid data key"
		err.IsRecoverable = false
	case "invalid_ttl":
		err.Message = "Invalid TTL"
		err.IsRecoverable = false
	case "unavailable":
		err.Message = "Server unavailable"
		err.IsRecoverable = true
	case "internal_server_error":
		err.Message = "Internal server error"
		err.IsRecoverable = true
	case "device_message_rate_exceeded":
		err.Message = "Device message rate exceeded"
		err.IsRecoverable = true
	case "topics_message_rate_exceeded":
		err.Message = "Topics message rate exceeded"
		err.IsRecoverable = true
	default:
		err.IsRecoverable = true
	}

	return err
}
