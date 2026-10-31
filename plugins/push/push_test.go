package push

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewNotification(t *testing.T) {
	notification := NewNotification("Test Title", "Test Body", PlatformAndroid)

	assert.Equal(t, "Test Title", notification.Title)
	assert.Equal(t, "Test Body", notification.Body)
	assert.Equal(t, PlatformAndroid, notification.Platform)
	assert.Equal(t, PriorityNormal, notification.Priority)
	assert.Equal(t, "default", notification.Sound)
}

func TestNotification_WithTokens(t *testing.T) {
	notification := NewNotification("Title", "Body", PlatformAndroid).
		WithTokens("token1", "token2", "token3")

	assert.Len(t, notification.Tokens, 3)
	assert.Contains(t, notification.Tokens, "token1")
	assert.Contains(t, notification.Tokens, "token2")
	assert.Contains(t, notification.Tokens, "token3")
}

func TestNotification_WithTopic(t *testing.T) {
	notification := NewNotification("Title", "Body", PlatformAndroid).
		WithTopic("news")

	assert.Equal(t, "news", notification.Topic)
}

func TestNotification_WithData(t *testing.T) {
	notification := NewNotification("Title", "Body", PlatformAndroid).
		WithData("key1", "value1").
		WithData("key2", "value2")

	assert.NotNil(t, notification.Data)
	assert.Equal(t, "value1", notification.Data["key1"])
	assert.Equal(t, "value2", notification.Data["key2"])
}

func TestNotification_WithHighPriority(t *testing.T) {
	notification := NewNotification("Title", "Body", PlatformAndroid).
		WithHighPriority()

	assert.Equal(t, PriorityHigh, notification.Priority)
}

func TestNotification_WithBadge(t *testing.T) {
	notification := NewNotification("Title", "Body", PlatformIOS).
		WithBadge(5)

	assert.Equal(t, 5, notification.Badge)
}

func TestNotification_WithSound(t *testing.T) {
	notification := NewNotification("Title", "Body", PlatformAndroid).
		WithSound("custom_sound.wav")

	assert.Equal(t, "custom_sound.wav", notification.Sound)
}

func TestNotification_WithImage(t *testing.T) {
	notification := NewNotification("Title", "Body", PlatformAndroid).
		WithImage("https://example.com/image.png")

	assert.Equal(t, "https://example.com/image.png", notification.Image)
}

func TestNotification_WithDeepLink(t *testing.T) {
	notification := NewNotification("Title", "Body", PlatformAndroid).
		WithDeepLink("myapp://screen/details")

	assert.Equal(t, "myapp://screen/details", notification.DeepLink)
}

func TestNotification_IsValid(t *testing.T) {
	tests := []struct {
		name        string
		notification *Notification
		wantErr     bool
		errMsg      string
	}{
		{
			name: "valid with tokens",
			notification: &Notification{
				Title:    "Title",
				Body:     "Body",
				Platform: PlatformAndroid,
				Tokens:   []string{"token1"},
			},
			wantErr: false,
		},
		{
			name: "valid with topic",
			notification: &Notification{
				Title:    "Title",
				Body:     "Body",
				Platform: PlatformIOS,
				Topic:    "news",
			},
			wantErr: false,
		},
		{
			name: "missing title",
			notification: &Notification{
				Title:    "",
				Body:     "Body",
				Platform: PlatformAndroid,
				Tokens:   []string{"token1"},
			},
			wantErr: true,
			errMsg:  "title is required",
		},
		{
			name: "missing body",
			notification: &Notification{
				Title:    "Title",
				Body:     "",
				Platform: PlatformAndroid,
				Tokens:   []string{"token1"},
			},
			wantErr: true,
			errMsg:  "body is required",
		},
		{
			name: "invalid platform",
			notification: &Notification{
				Title:    "Title",
				Body:     "Body",
				Platform: "windows",
				Tokens:   []string{"token1"},
			},
			wantErr: true,
			errMsg:  "invalid platform",
		},
		{
			name: "missing tokens and topic",
			notification: &Notification{
				Title:    "Title",
				Body:     "Body",
				Platform: PlatformAndroid,
			},
			wantErr: true,
			errMsg:  "either tokens or topic must be specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.notification.IsValid()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestError_Error(t *testing.T) {
	err := &Error{
		Code:    "INVALID_TOKEN",
		Message: "The device token is invalid",
	}

	assert.Equal(t, "The device token is invalid", err.Error())
}

func TestResponse_Structure(t *testing.T) {
	response := &Response{
		Success:   true,
		MessageID: "message-123",
		Token:     "device-token",
		Error:     nil,
		Provider:  ProviderTypeFCM,
	}

	assert.True(t, response.Success)
	assert.Equal(t, "message-123", response.MessageID)
	assert.Equal(t, "device-token", response.Token)
	assert.Nil(t, response.Error)
	assert.Equal(t, ProviderTypeFCM, response.Provider)
}

func TestResponse_WithError(t *testing.T) {
	pushErr := &Error{
		Code:          "INVALID_TOKEN",
		Message:       "Token is invalid",
		IsRecoverable: false,
		InvalidToken:  true,
	}

	response := &Response{
		Success:  false,
		Token:    "bad-token",
		Error:    pushErr,
		Provider: ProviderTypeAPNs,
	}

	assert.False(t, response.Success)
	assert.NotNil(t, response.Error)
	assert.Equal(t, "INVALID_TOKEN", response.Error.Code)
	assert.False(t, response.Error.IsRecoverable)
	assert.True(t, response.Error.InvalidToken)
}

func TestBatchResponse_Structure(t *testing.T) {
	responses := []*Response{
		{Success: true, Token: "token1"},
		{Success: false, Token: "token2", Error: &Error{Code: "ERROR"}},
		{Success: true, Token: "token3"},
	}

	batch := &BatchResponse{
		SuccessCount: 2,
		FailureCount: 1,
		Responses:    responses,
		Provider:     ProviderTypeFCM,
	}

	assert.Equal(t, 2, batch.SuccessCount)
	assert.Equal(t, 1, batch.FailureCount)
	assert.Len(t, batch.Responses, 3)
	assert.Equal(t, ProviderTypeFCM, batch.Provider)
}

func TestError_IsRecoverable(t *testing.T) {
	recoverableErr := &Error{
		Code:          "UNAVAILABLE",
		Message:       "Service unavailable",
		IsRecoverable: true,
	}
	assert.True(t, recoverableErr.IsRecoverable)

	nonRecoverableErr := &Error{
		Code:          "INVALID_TOKEN",
		Message:       "Bad device token",
		IsRecoverable: false,
		InvalidToken:  true,
	}
	assert.False(t, nonRecoverableErr.IsRecoverable)
	assert.True(t, nonRecoverableErr.InvalidToken)
}
