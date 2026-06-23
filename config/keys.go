package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/argon2"
	"golang.org/x/term"
)

var (
	sessionKey     []byte
	sessionKeyOnce sync.Once
	sessionKeyMu   sync.Mutex
)

// GetAPIKey decrypts and returns an API key by slot name.
func GetAPIKey(slot string) (string, error) {
	key, err := getOrDeriveSessionKey()
	if err == nil {
		if val, err := loadAndDecryptKey(slot, key); err == nil {
			return val, nil
		}
	}

	// Fallback to baked-in keys if no user key exists
	if fallback, ok := GetFallbackKey(slot); ok {
		return fallback, nil
	}

	return "", fmt.Errorf("no key found for %s and no fallback available", slot)
}

// SetAPIKey encrypts and stores an API key.
func SetAPIKey(slot, value string) error {
	key, err := getOrDeriveSessionKey()
	if err != nil {
		return err
	}
	return encryptAndStoreKey(slot, value, key)
}

// ResetSessionKey clears the cached passphrase (forces re-prompt on next use).
func ResetSessionKey() {
	sessionKeyMu.Lock()
	defer sessionKeyMu.Unlock()
	sessionKey = nil
	sessionKeyOnce = sync.Once{} // allow re-derivation
}

func getOrDeriveSessionKey() ([]byte, error) {
	sessionKeyMu.Lock()
	defer sessionKeyMu.Unlock()
	if sessionKey != nil {
		return sessionKey, nil
	}

	salt, err := loadOrCreateSalt()
	if err != nil {
		return nil, err
	}

	fmt.Print("Enter Goo passphrase: ")
	passphrase, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return nil, err
	}

	derived := argon2.IDKey(passphrase, salt, 3, 64*1024, 4, 32)
	sessionKey = derived
	return derived, nil
}

func loadOrCreateSalt() ([]byte, error) {
	saltPath := filepath.Join(GooConfigDir(), "salt")
	data, err := os.ReadFile(saltPath)
	if err == nil {
		return data, nil
	}
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(GooConfigDir(), 0700); err != nil {
		return nil, err
	}
	return salt, os.WriteFile(saltPath, salt, 0600)
}

func encryptAndStoreKey(slot, value string, masterKey []byte) error {
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(value), nil)

	store := loadKeyStore()
	store[slot] = base64.StdEncoding.EncodeToString(ciphertext)
	return saveKeyStore(store)
}

func loadAndDecryptKey(slot string, masterKey []byte) (string, error) {
	store := loadKeyStore()
	encoded, ok := store[slot]
	if !ok {
		return "", errors.New("key not found for slot: " + slot)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", errors.New("decryption failed: wrong passphrase?")
	}
	return string(plaintext), nil
}

func loadKeyStore() map[string]string {
	store := map[string]string{}
	data, err := os.ReadFile(keyStorePath())
	if err != nil {
		return store
	}
	_ = json.Unmarshal(data, &store)
	return store
}

func saveKeyStore(store map[string]string) error {
	if err := os.MkdirAll(GooConfigDir(), 0700); err != nil {
		return err
	}
	data, _ := json.Marshal(store)
	return os.WriteFile(keyStorePath(), data, 0600)
}

func keyStorePath() string { return filepath.Join(GooConfigDir(), "keys.enc") }

// ListSlots returns all stored key slot names (without decrypting values).
func ListSlots() []string {
	store := loadKeyStore()
	slots := make([]string, 0, len(store))
	for k := range store {
		slots = append(slots, k)
	}
	return slots
}

// HasKey returns true if a slot has been set.
func HasKey(slot string) bool {
	store := loadKeyStore()
	_, ok := store[slot]
	return ok
}

// DeleteKey removes a key slot.
func DeleteKey(slot string) error {
	store := loadKeyStore()
	delete(store, slot)
	return saveKeyStore(store)
}
