package jwtx

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// mockStore implements Store interface for testing
type mockStore struct {
	data map[string]string
	mu   sync.RWMutex
}

func newMockStore() *mockStore {
	return &mockStore{
		data: make(map[string]string),
	}
}

func (m *mockStore) Set(key string, value string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return nil
}

func (m *mockStore) Get(key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if val, ok := m.data[key]; ok {
		return val, nil
	}
	return "", errors.New("key not found")
}

func (m *mockStore) Del(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func TestInit(t *testing.T) {
	// Test with default values
	cfg := Config{
		Secret: "test-secret",
	}
	Init(cfg)
	current := DefaultManager().Config()

	if current.Secret != "test-secret" {
		t.Errorf("Expected secret to be 'test-secret', got '%s'", current.Secret)
	}
	if current.AccessTokenTTL != 15*time.Minute {
		t.Errorf("Expected default access token TTL to be 15 minutes, got %v", current.AccessTokenTTL)
	}
	if current.RefreshTokenTTL != 7*24*time.Hour {
		t.Errorf("Expected default refresh token TTL to be 7 days, got %v", current.RefreshTokenTTL)
	}

	// Test with custom values
	customCfg := Config{
		Secret:         "custom-secret",
		AccessTokenTTL: time.Hour,
		RefreshTokenTTL: 30 * 24 * time.Hour,
	}
	Init(customCfg)
	current = DefaultManager().Config()

	if current.Secret != "custom-secret" {
		t.Errorf("Expected secret to be 'custom-secret', got '%s'", current.Secret)
	}
	if current.AccessTokenTTL != time.Hour {
		t.Errorf("Expected custom access token TTL to be 1 hour, got %v", current.AccessTokenTTL)
	}
	if current.RefreshTokenTTL != 30*24*time.Hour {
		t.Errorf("Expected custom refresh token TTL to be 30 days, got %v", current.RefreshTokenTTL)
	}
}

func TestGenerateTokenPair(t *testing.T) {
	Init(Config{
		Secret: "test-secret",
		Store:  newMockStore(),
	})

	userID := "user123"
	username := "testuser"
	deviceID := "device456"

	pair, err := GenerateTokenPair(userID, username, deviceID)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	if pair.AccessToken == "" {
		t.Error("AccessToken should not be empty")
	}
	if pair.RefreshToken == "" {
		t.Error("RefreshToken should not be empty")
	}

	// Verify tokens are different
	if pair.AccessToken == pair.RefreshToken {
		t.Error("AccessToken and RefreshToken should be different")
	}
}

func TestValidateAccessToken(t *testing.T) {
	store := newMockStore()
	Init(Config{
		Secret: "test-secret",
		Store:  store,
	})

	userID := "user123"
	username := "testuser"
	deviceID := "device456"

	// Generate valid token
	pair, err := GenerateTokenPair(userID, username, deviceID)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	// Validate access token
	claims, err := ValidateAccessToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ValidateAccessToken() error = %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("Expected UserID %s, got %s", userID, claims.UserID)
	}
	if claims.Username != username {
		t.Errorf("Expected Username %s, got %s", username, claims.Username)
	}
	if claims.DeviceID != deviceID {
		t.Errorf("Expected DeviceID %s, got %s", deviceID, claims.DeviceID)
	}
	if claims.Type != TokenTypeAccess {
		t.Errorf("Expected token type %s, got %s", TokenTypeAccess, claims.Type)
	}
}

func TestValidateAccessTokenErrors(t *testing.T) {
	store := newMockStore()
	Init(Config{
		Secret: "test-secret",
		Store:  store,
	})

	userID := "user123"
	username := "testuser"
	deviceID := "device456"

	// Test invalid token
	_, err := ValidateAccessToken("invalid.token")
	if err == nil {
		t.Error("ValidateAccessToken() should return error for invalid token")
	}

	// Test expired token
	_, err = GenerateTokenPair(userID, username, deviceID)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	// Manually expire the token by setting a very short TTL
	Init(Config{
		Secret:         "test-secret",
		AccessTokenTTL: time.Millisecond,
		Store:          store,
	})

	expiredPair, err := GenerateTokenPair(userID, username, deviceID)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	_, err = ValidateAccessToken(expiredPair.AccessToken)
	if err == nil {
		t.Error("ValidateAccessToken() should return error for expired token")
	} else if err.Error() != "token expired" {
		t.Errorf("Expected 'token expired' error, got %v", err)
	}

	// Test revoked token
	Init(Config{
		Secret: "test-secret",
		Store:  store,
	})

	validPair, err := GenerateTokenPair(userID, username, deviceID)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	// Add token to blacklist - need to extract TokenID first
	claims, _ := decodeClaims(validPair.AccessToken)
	store.Set("blacklist:"+claims.TokenID, "1", time.Hour)

	_, err = ValidateAccessToken(validPair.AccessToken)
	if err == nil {
		t.Error("ValidateAccessToken() should return error for revoked token")
	} else if err.Error() != "token revoked" {
		t.Errorf("Expected 'token revoked' error, got %v", err)
	}

	// Test wrong token type (refresh token as access token)
	_, err = ValidateAccessToken(validPair.RefreshToken)
	if err == nil {
		t.Error("ValidateAccessToken() should return error for refresh token")
	} else if err.Error() != "invalid token type" {
		t.Errorf("Expected 'invalid token type' error, got %v", err)
	}
}

func TestRefresh(t *testing.T) {
	store := newMockStore()
	Init(Config{
		Secret: "test-secret",
		Store:  store,
	})

	userID := "user123"
	username := "testuser"
	deviceID := "device456"

	// Generate initial token pair
	pair, err := GenerateTokenPair(userID, username, deviceID)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	// Refresh tokens
	newPair, err := Refresh(pair.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	if newPair.AccessToken == "" {
		t.Error("New AccessToken should not be empty")
	}
	if newPair.RefreshToken == "" {
		t.Error("New RefreshToken should not be empty")
	}

	// Verify old refresh token is deleted
	refreshClaims, _ := decodeClaims(pair.RefreshToken)
	_, err = store.Get("refresh:" + refreshClaims.TokenID)
	if err == nil {
		t.Error("Old refresh token should be deleted from store")
	}

	// Verify new tokens are different from old ones
	if newPair.AccessToken == pair.AccessToken {
		t.Error("New access token should be different from old access token")
	}
	if newPair.RefreshToken == pair.RefreshToken {
		t.Error("New refresh token should be different from old refresh token")
	}
}

func TestRefreshErrors(t *testing.T) {
	store := newMockStore()
	Init(Config{
		Secret: "test-secret",
		Store:  store,
	})

	userID := "user123"
	username := "testuser"
	deviceID := "device456"

	// Test invalid refresh token
	_, err := Refresh("invalid.token")
	if err == nil {
		t.Error("Refresh() should return error for invalid token")
	}

	// Test access token as refresh token
	pair, err := GenerateTokenPair(userID, username, deviceID)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	_, err = Refresh(pair.AccessToken)
	if err == nil {
		t.Error("Refresh() should return error for access token")
	} else if err.Error() != "invalid refresh token" {
		t.Errorf("Expected 'invalid refresh token' error, got %v", err)
	}

	// Test expired refresh token
	Init(Config{
		Secret:         "test-secret",
		RefreshTokenTTL: time.Millisecond,
		Store:          store,
	})

	expiredPair, err := GenerateTokenPair(userID, username, deviceID)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	_, err = Refresh(expiredPair.RefreshToken)
	if err == nil {
		t.Error("Refresh() should return error for expired token")
	} else if err.Error() != "refresh token expired" {
		t.Errorf("Expected 'refresh token expired' error, got %v", err)
	}

	// Test refresh token not in store
	Init(Config{
		Secret: "test-secret",
		Store:  store,
	})

	validPair, err := GenerateTokenPair(userID, username, deviceID)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	// Delete refresh token from store
	refreshClaims, _ := decodeClaims(validPair.RefreshToken)
	store.Del("refresh:" + refreshClaims.TokenID)

	_, err = Refresh(validPair.RefreshToken)
	if err == nil {
		t.Error("Refresh() should return error for token not in store")
	} else if err.Error() != "refresh token invalid" {
		t.Errorf("Expected 'refresh token invalid' error, got %v", err)
	}
}

func TestTokenVersioning(t *testing.T) {
	store := newMockStore()
	Init(Config{
		Secret: "test-secret",
		Store:  store,
	})

	userID := "user123"
	username := "testuser"
	deviceID := "device456"

	// Set initial version
	store.Set("user:version:"+userID, "1", time.Hour)

	// Generate token
	pair, err := GenerateTokenPair(userID, username, deviceID)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	// Validate token should work
	_, err = ValidateAccessToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ValidateAccessToken() error = %v", err)
	}

	// Update user version
	store.Set("user:version:"+userID, "2", time.Hour)

	// Token should now be invalid
	_, err = ValidateAccessToken(pair.AccessToken)
	if err == nil {
		t.Error("ValidateAccessToken() should return error when user version changes")
	} else if err.Error() != "token invalid" {
		t.Errorf("Expected 'token invalid' error, got %v", err)
	}
}

func TestEncryptDecrypt(t *testing.T) {
	Init(Config{
		Secret: "test-secret",
	})

	// Test encrypt/decrypt cycle
	data := []byte("test data for encryption")

	encrypted, err := encrypt(data)
	if err != nil {
		t.Fatalf("encrypt() error = %v", err)
	}

	if encrypted == "" {
		t.Error("encrypted string should not be empty")
	}

	decrypted, err := decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt() error = %v", err)
	}

	if string(decrypted) != string(data) {
		t.Errorf("Expected decrypted data '%s', got '%s'", string(data), string(decrypted))
	}
}

func TestEncodeDecodeClaims(t *testing.T) {
	Init(Config{
		Secret: "test-secret",
	})

	claims := Claims{
		UserID:   "user123",
		Username: "testuser",
		DeviceID: "device456",
		TokenID:  "token789",
		Version:  1,
		Type:     TokenTypeAccess,
		IssuedAt: time.Now(),
		ExpireAt: time.Now().Add(time.Hour),
	}

	// Encode claims
	token, err := encodeClaims(claims)
	if err != nil {
		t.Fatalf("encodeClaims() error = %v", err)
	}

	if token == "" {
		t.Error("token should not be empty")
	}

	// Decode claims
	decodedClaims, err := decodeClaims(token)
	if err != nil {
		t.Fatalf("decodeClaims() error = %v", err)
	}

	if decodedClaims.UserID != claims.UserID {
		t.Errorf("Expected UserID %s, got %s", claims.UserID, decodedClaims.UserID)
	}
	if decodedClaims.Username != claims.Username {
		t.Errorf("Expected Username %s, got %s", claims.Username, decodedClaims.Username)
	}
	if decodedClaims.DeviceID != claims.DeviceID {
		t.Errorf("Expected DeviceID %s, got %s", claims.DeviceID, decodedClaims.DeviceID)
	}
	if decodedClaims.TokenID != claims.TokenID {
		t.Errorf("Expected TokenID %s, got %s", claims.TokenID, decodedClaims.TokenID)
	}
	if decodedClaims.Version != claims.Version {
		t.Errorf("Expected Version %d, got %d", claims.Version, decodedClaims.Version)
	}
	if decodedClaims.Type != claims.Type {
		t.Errorf("Expected Type %s, got %s", claims.Type, decodedClaims.Type)
	}
}

func TestConcurrentTokenOperations(t *testing.T) {
	store := newMockStore()
	Init(Config{
		Secret: "test-secret",
		Store:  store,
	})

	const numGoroutines = 100
	results := make(chan error, numGoroutines)

	// Test concurrent token generation
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			userID := "user" + string(rune(id))
			username := "user" + string(rune(id))
			deviceID := "device" + string(rune(id))

			pair, err := GenerateTokenPair(userID, username, deviceID)
			if err != nil {
				results <- err
				return
			}

			// Validate the generated token
			_, err = ValidateAccessToken(pair.AccessToken)
			results <- err
		}(i)
	}

	// Collect results
	for i := 0; i < numGoroutines; i++ {
		if err := <-results; err != nil {
			t.Errorf("Concurrent operation failed: %v", err)
		}
	}
}

func TestLogout(t *testing.T) {
	store := newMockStore()
	Init(Config{
		Secret: "test-secret",
		Store:  store,
	})

	userID := "user123"
	username := "testuser"
	deviceID := "device456"

	// Generate token pair
	pair, err := GenerateTokenPair(userID, username, deviceID)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	// Validate token works initially
	_, err = ValidateAccessToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ValidateAccessToken() error = %v", err)
	}

	// Logout user
	err = Logout(pair.AccessToken)
	if err != nil {
		t.Fatalf("Logout() error = %v", err)
	}

	// Token should now be invalid
	_, err = ValidateAccessToken(pair.AccessToken)
	if err == nil {
		t.Error("ValidateAccessToken() should return error after logout")
	} else if err.Error() != "token revoked" {
		t.Errorf("Expected 'token revoked' error, got %v", err)
	}
}

func TestManager_Isolation(t *testing.T) {
	storeA := newMockStore()
	storeB := newMockStore()
	managerA := NewManager(Config{
		Secret: "secret-a",
		Store:  storeA,
	})
	managerB := NewManager(Config{
		Secret: "secret-b",
		Store:  storeB,
	})

	pairA, err := managerA.GenerateTokenPair("user-a", "alice", "device-a")
	if err != nil {
		t.Fatalf("managerA.GenerateTokenPair() error = %v", err)
	}
	if _, err := managerA.ValidateAccessToken(pairA.AccessToken); err != nil {
		t.Fatalf("managerA.ValidateAccessToken() error = %v", err)
	}
	if _, err := managerB.ValidateAccessToken(pairA.AccessToken); err == nil {
		t.Fatal("expected managerB validation to fail for managerA token")
	}
}
