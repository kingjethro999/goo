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
)

// GetAPIKey returns a key by slot. Priority:
//  1. User's personal key — decrypted automatically using the machine key (no passphrase)
//  2. Baked-in fallback key — available to all users out of the box
func GetAPIKey(slot string) (string, error) {
	// Try user's personal key first (zero-friction — machine key auto-decrypts)
	if HasKey(slot) {
		mk, err := loadOrCreateMachineKey()
		if err == nil {
			if val, err := loadAndDecryptKey(slot, mk); err == nil {
				return val, nil
			}
		}
	}

	// No personal key or decryption failed — use baked-in fallback
	if fallback, ok := GetFallbackKey(slot); ok {
		return fallback, nil
	}

	return "", fmt.Errorf("no key configured for %s — run: goo config set-key %s", slot, slot)
}

// SetAPIKey encrypts and stores a personal API key using the machine key.
// No passphrase required — the machine key is generated automatically.
func SetAPIKey(slot, value string) error {
	mk, err := loadOrCreateMachineKey()
	if err != nil {
		return err
	}
	return encryptAndStoreKey(slot, value, mk)
}

// loadOrCreateMachineKey returns the 32-byte machine-specific encryption key.
// It is generated once on first use and stored at ~/.config/goo/machine.key.
// This key is unique per machine so keys.enc is not portable — by design.
func loadOrCreateMachineKey() ([]byte, error) {
	path := filepath.Join(GooConfigDir(), "machine.key")
	data, err := os.ReadFile(path)
	if err == nil && len(data) == 32 {
		return data, nil
	}

	// Generate a new machine key
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(GooConfigDir(), 0700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, key, 0600); err != nil {
		return nil, err
	}
	return key, nil
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
		return "", errors.New("decryption failed — machine key mismatch?")
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
