package config

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
)

var fallbackKeys = map[string]string{
	"groq":      "e9bf6c467c6c477ab16094e3301626271b013cd210cdfacb94d2d673fe3e350f165cf0366cc4b7ad93a15c49d7da32921a8bdb47651f98f65c6c7e72a2a0426516b74949e80d121ec0ef0603f302ff95531da4c1",
	"tavily":    "1c88efcc463b6476fdc67bfaf47a9607a86e22a6bf66089c1300fd1a494be1b09fdc24625258e8ccfebdfefaea8c8a30d8ce20b5763bd552d9f160016e8d2a8098e712748a201f412d957c43d31d97b5e467fc8034bf",
	"exa":       "1d03f05c45be8a2c23070162e4903911232932dae66cea8325bb2844cb39853f05cd4aeed8b4143df32fb35b1bf9638b32cddbc737a949c60368ccf6ff45bd46",
	"firecrawl": "c96c55205edea87c0edbfe762393d5af35fe0e32922da756be8531b95604c9326cc2efc1ed4b44f59983b3a5b134e0b57386b6d9ceaebe600113a2906a313b",
}

// GetFallbackKey provides a baked-in default API key if the user hasn't set one.
func GetFallbackKey(slot string) (string, bool) {
	enc, ok := fallbackKeys[slot]
	if !ok {
		return "", false
	}
	masterKey := []byte("GooFallbackKey00GooFallbackKey00") // 32 bytes
	ciphertext, _ := hex.DecodeString(enc)
	block, _ := aes.NewCipher(masterKey)
	gcm, _ := cipher.NewGCM(block)
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", false
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", false
	}
	return string(plaintext), true
}
