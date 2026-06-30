package config

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
)

var fallbackKeys = map[string]string{
	"groq":      "c5aeeea8bd09cfbd0223a1a936a56cd25541eaf5629607eb0517e2ead6649074b83d10a135d8b86183ee90d2f1747bd9a0a41be815477d295f36ed13c40edcaca12b5549317016424958ef19c1c5299d72a88141",
	"tavily":    "2b29e397a1b68bc56d05b4e3268c70ff2bd552f6dd3a4836c7f9bd438eab3a86936040eaf00f497b3c14ce3717be49de7384935aa177638eb21cc985ef994847fac9e90ef4b9b7fb0a831ec2d75e47352b928cc12569",
	"exa":       "6d2aa701509d2ce25efa4f95477f27d8bed41f47b1d1fad276bf11c53a5d68ea9d212106bf2b761329a17cda626a24313eb2c1b74dda9abeb8d1d77ec36fb79e",
	"firecrawl": "2f8321cc62f932c441dd759ccfacfe8a853e2ac50aa9c71f2c2d7ab209763c96214e9c7d717529e04cd79eff687929caed9567bd2504485dc3a1ae400f3e25",
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
