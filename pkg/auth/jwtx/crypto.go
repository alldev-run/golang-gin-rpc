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
func encrypt(data []byte) (string, error) {

	key := sha256.Sum256([]byte(config.Secret))

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
func decrypt(token string) ([]byte, error) {

	key := sha256.Sum256([]byte(config.Secret))

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
func encodeClaims(c Claims) (string, error) {

	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}

	return encrypt(data)
}

// decodeClaims decrypts and unmarshals a token string into claims.
func decodeClaims(token string) (*Claims, error) {

	data, err := decrypt(token)
	if err != nil {
		return nil, err
	}

	var c Claims

	err = json.Unmarshal(data, &c)

	return &c, err
}
