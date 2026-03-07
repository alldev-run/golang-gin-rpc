package jwt

import (
	"crypto/aes"      // AES加密算法实现
	"crypto/cipher"   // 加密算法接口，支持各种加密模式
	"crypto/rand"     // 加密安全的随机数生成器
	"crypto/sha256"   // SHA256哈希算法
	"encoding/base64" // Base64编码，用于token的传输
	"encoding/json"   // JSON序列化和反序列化
	"errors"          // 标准错误处理
	"fmt"             // 格式化字符串输出
	"io"              // I/O操作接口
	"time"            // 时间处理
)

var (
	// jwtSecret JWT签名密钥，需要在应用启动时通过InitJWT函数设置
	jwtSecret string
	// iv AES加密的初始化向量，16字节长度，用于增强加密安全性
	// 注意：在实际生产环境中，这个IV应该每次都随机生成，这里为了简化使用固定值
	iv = []byte{0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef, 0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef}
)

// Claims JWT载荷结构体，包含用户信息和时间戳
type Claims struct {
	UserID   string    `json:"user_id"`   // 用户唯一标识
	Username string    `json:"username"`  // 用户名
	ExpireAt time.Time `json:"expire_at"` // token过期时间
	IssuedAt time.Time `json:"issued_at"` // token签发时间
}

// InitJWT 初始化JWT密钥
// 参数: secret - 用于加密签名的密钥字符串
func InitJWT(secret string) {
	jwtSecret = secret
}

// EncodeJwt 生成JWT token
// 参数: 
//   - userID: 用户ID
//   - username: 用户名
// 返回值:
//   - string: Base64编码的JWT token
//   - error: 错误信息
func EncodeJwt(userID, username string) (string, error) {
	// 检查密钥是否已经初始化
	if jwtSecret == "" {
		return "", errors.New("JWT secret not initialized")
	}

	// 创建Claims对象，包含用户信息和时间戳
	claims := Claims{
		UserID:   userID,
		Username: username,
		ExpireAt: time.Now().Add(24 * time.Hour), // 设置24小时后过期
		IssuedAt: time.Now(),                     // 设置签发时间为当前时间
	}

	// 将Claims对象序列化为JSON字符串
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	// 使用SHA256对密钥进行哈希处理，生成32字节的AES密钥
	key := sha256.Sum256([]byte(jwtSecret))
	
	// 创建AES加密器，使用32字节密钥（AES-256）
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}

	// 创建GCM（Galois/Counter Mode）加密模式
	// GCM提供认证加密，同时保证机密性和完整性
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// 生成随机nonce（数字一次性编号）
	// nonce用于确保相同明文每次加密都产生不同密文
	nonce := make([]byte, gcm.NonceSize()) // GCM模式下nonce通常为12字节
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// 使用GCM模式加密数据
	// Seal函数会返回: nonce + ciphertext + authentication tag
	ciphertext := gcm.Seal(nonce, nonce, claimsJSON, nil)

	// 将加密结果进行Base64编码，便于HTTP传输和存储
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecodeJwt 解析JWT token
// 参数:
//   - tokenString: Base64编码的JWT token字符串
// 返回值:
//   - *Claims: 解析后的用户信息结构体指针
//   - error: 错误信息
func DecodeJwt(tokenString string) (*Claims, error) {
	// 检查密钥是否已经初始化
	if jwtSecret == "" {
		return nil, errors.New("JWT secret not initialized")
	}

	// 对Base64编码的token进行解码
	ciphertext, err := base64.StdEncoding.DecodeString(tokenString)
	if err != nil {
		return nil, fmt.Errorf("invalid token format: %w", err)
	}

	// 使用相同的密钥创建AES加密器
	key := sha256.Sum256([]byte(jwtSecret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}

	// 创建GCM解密器
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// 检查密文长度是否足够包含nonce
	// 最小长度 = nonce长度 + 至少1字节的密文
	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("token too short")
	}

	// 从密文中分离nonce和实际加密数据
	// 前nonceSize()字节是nonce，剩余部分是加密的claims数据
	nonce := ciphertext[:gcm.NonceSize()]
	ciphertext = ciphertext[gcm.NonceSize():]

	// 使用GCM模式解密数据
	// Open函数会自动验证authentication tag，确保数据完整性
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("token decryption failed: %w", err)
	}

	// 将解密后的JSON数据反序列化为Claims对象
	var claims Claims
	if err := json.Unmarshal(plaintext, &claims); err != nil {
		return nil, fmt.Errorf("invalid claims: %w", err)
	}

	// 检查token是否已过期
	if time.Now().After(claims.ExpireAt) {
		return nil, errors.New("token expired")
	}

	// 返回解析成功的Claims对象
	return &claims, nil
}
