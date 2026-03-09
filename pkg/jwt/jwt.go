package jwt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

// Config JWT 配置（从外部读取）
type Config struct {
	Secret string `json:"secret" yaml:"secret" mapstructure:"secret"` // 支持 json/yaml/viper 等
}

// global config（单例方式加载一次）
var jwtConfig Config

// Init 初始化 JWT 配置（建议在 main 或初始化阶段调用一次）
func Init(cfg Config) error {
	if cfg.Secret == "" {
		return errors.New("jwt secret cannot be empty")
	}
	jwtConfig = cfg
	return nil
}

// 内部获取密钥（避免直接访问全局变量）
func getSecret() string {
	return jwtConfig.Secret
}

// Claims 保持不变
type Claims struct {
	UserID   string    `json:"user_id"`
	Username string    `json:"username"`
	ExpireAt time.Time `json:"expire_at"`
	IssuedAt time.Time `json:"issued_at"`
}

// EncodeJwt 生成 token
func EncodeJwt(userID, username string) (string, error) {
	secret := getSecret()
	if secret == "" {
		return "", errors.New("jwt secret not initialized")
	}

	claims := Claims{
		UserID:   userID,
		Username: username,
		ExpireAt: time.Now().Add(24 * time.Hour),
		IssuedAt: time.Now(),
	}

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, claimsJSON, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecodeJwt 解析 token（逻辑同上，只改了 secret 获取方式）
func DecodeJwt(tokenString string) (*Claims, error) {
	secret := getSecret()
	if secret == "" {
		return nil, errors.New("jwt secret not initialized")
	}

	// 以下解密逻辑保持不变，只替换 jwtSecret 为 secret
	ciphertext, err := base64.StdEncoding.DecodeString(tokenString)
	if err != nil {
		return nil, fmt.Errorf("invalid token format: %w", err)
	}

	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("token too short")
	}

	nonce := ciphertext[:gcm.NonceSize()]
	ciphertext = ciphertext[gcm.NonceSize():]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("token decryption failed: %w", err)
	}

	var claims Claims
	if err := json.Unmarshal(plaintext, &claims); err != nil {
		return nil, fmt.Errorf("invalid claims: %w", err)
	}

	if time.Now().After(claims.ExpireAt) {
		return nil, errors.New("token expired")
	}

	return &claims, nil
}
