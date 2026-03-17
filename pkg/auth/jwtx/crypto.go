package jwtx

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
)

// encrypt encrypts data using AES-GCM with the configured secret key.
func (m *Manager) encrypt(data []byte) (string, error) {
	cfg := m.Config()
	key := sha256.Sum256([]byte(cfg.Secret))

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())

	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt decrypts an encrypted token using AES-GCM.
func (m *Manager) decrypt(token string) ([]byte, error) {
	cfg := m.Config()
	key := sha256.Sum256([]byte(cfg.Secret))

	data, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(data) < gcm.NonceSize() {
		return nil, errors.New("token invalid")
	}

	nonce := data[:gcm.NonceSize()]
	ciphertext := data[gcm.NonceSize():]

	return gcm.Open(nil, nonce, ciphertext, nil)
}

// encodeClaims marshals and encrypts claims into a token string.
func (m *Manager) encodeClaims(c Claims) (string, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}

	return m.encrypt(data)
}

// decodeClaims decrypts and unmarshals a token string into claims.
func (m *Manager) decodeClaims(token string) (*Claims, error) {
	data, err := m.decrypt(token)
	if err != nil {
		return nil, err
	}

	var c Claims

	err = json.Unmarshal(data, &c)

	return &c, err
}

func encrypt(data []byte) (string, error) {
	return DefaultManager().encrypt(data)
}

func decrypt(token string) ([]byte, error) {
	return DefaultManager().decrypt(token)
}

func encodeClaims(c Claims) (string, error) {
	return DefaultManager().encodeClaims(c)
}

func decodeClaims(token string) (*Claims, error) {
	return DefaultManager().decodeClaims(token)
}
