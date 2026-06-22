package config

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "encoding/json"
    "errors"
    "io"
    "os"
    "path/filepath"
    "sync"

    "golang.org/x/crypto/argon2"
    "golang.org/x/term"
    "fmt"
)

var (
    sessionKey     []byte
    sessionKeyOnce sync.Once
    sessionKeyMu   sync.Mutex
)

// GetAPIKey decrypts and returns an API key by slot name.
func GetAPIKey(slot string) (string, error) {
    key, err := getOrDeriveSessionKey()
    if err != nil {
        return "", err
    }
    return loadAndDecryptKey(slot, key)
}

// SetAPIKey encrypts and stores an API key.
func SetAPIKey(slot, value string) error {
    key, err := getOrDeriveSessionKey()
    if err != nil {
        return err
    }
    return encryptAndStoreKey(slot, value, key)
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
    saltPath := filepath.Join(gooConfigDir(), "salt")
    data, err := os.ReadFile(saltPath)
    if err == nil {
        return data, nil
    }
    salt := make([]byte, 32)
    if _, err := rand.Read(salt); err != nil {
        return nil, err
    }
    if err := os.MkdirAll(gooConfigDir(), 0700); err != nil {
        return nil, err
    }
    return salt, os.WriteFile(saltPath, salt, 0600)
}

func encryptAndStoreKey(slot, value string, masterKey []byte) error {
    block, _ := aes.NewCipher(masterKey)
    gcm, _ := cipher.NewGCM(block)
    nonce := make([]byte, gcm.NonceSize())
    io.ReadFull(rand.Reader, nonce)
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
    block, _ := aes.NewCipher(masterKey)
    gcm, _ := cipher.NewGCM(block)
    nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
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
    json.Unmarshal(data, &store)
    return store
}

func saveKeyStore(store map[string]string) error {
    data, _ := json.Marshal(store)
    return os.WriteFile(keyStorePath(), data, 0600)
}

func keyStorePath() string { return filepath.Join(gooConfigDir(), "keys.enc") }
func gooConfigDir() string {
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".config", "goo")
}
