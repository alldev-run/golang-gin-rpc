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

func encodeClaims(c Claims) (string, error) {

	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}

	return encrypt(data)
}

func decodeClaims(token string) (*Claims, error) {

	data, err := decrypt(token)
	if err != nil {
		return nil, err
	}

	var c Claims

	err = json.Unmarshal(data, &c)

	return &c, err
}
