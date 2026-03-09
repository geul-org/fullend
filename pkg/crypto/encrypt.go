package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"io"
)

// @func encrypt
// @description 평문을 AES-256-GCM으로 암호화한다

type EncryptRequest struct {
	Plaintext string
	Key       string // 32바이트 hex
}

type EncryptResponse struct {
	Ciphertext string // base64 인코딩
}

func Encrypt(req EncryptRequest) (EncryptResponse, error) {
	keyBytes, err := hex.DecodeString(req.Key)
	if err != nil {
		return EncryptResponse{}, err
	}
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return EncryptResponse{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return EncryptResponse{}, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return EncryptResponse{}, err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(req.Plaintext), nil)
	return EncryptResponse{Ciphertext: base64.StdEncoding.EncodeToString(sealed)}, nil
}
