package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// @func decrypt
// @description AES-256-GCM 암호문을 복호화한다

type DecryptRequest struct {
	Ciphertext string // base64 인코딩
	Key        string // 32바이트 hex
}

type DecryptResponse struct {
	Plaintext string
}

func Decrypt(req DecryptRequest) (DecryptResponse, error) {
	data, err := base64.StdEncoding.DecodeString(req.Ciphertext)
	if err != nil {
		return DecryptResponse{}, err
	}
	keyBytes, err := hex.DecodeString(req.Key)
	if err != nil {
		return DecryptResponse{}, err
	}
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return DecryptResponse{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return DecryptResponse{}, err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return DecryptResponse{}, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return DecryptResponse{}, err
	}
	return DecryptResponse{Plaintext: string(plaintext)}, nil
}
