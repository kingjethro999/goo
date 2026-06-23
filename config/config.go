package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/BurntSushi/toml"
)

// Schema mirrors config.toml structure
type Schema struct {
	General struct {
		DefaultModel    string `toml:"default_model"`
		Theme           string `toml:"theme"`
		HistoryLimit    int    `toml:"history_limit"`
		AutoFollowup    bool   `toml:"auto_followup"`
		Stream          bool   `toml:"stream"`
		DefaultProvider string `toml:"default_provider"`
	} `toml:"general"`
	AI struct {
		MaxTokens    int     `toml:"max_tokens"`
		Temperature  float64 `toml:"temperature"`
		SystemPrompt string  `toml:"system_prompt"`
	} `toml:"ai"`
	GitHub struct {
		Username    string `toml:"username"`
		DefaultRepo string `toml:"default_repo"`
	} `toml:"github"`
	Tasks struct {
		StoragePath     string `toml:"storage_path"`
		DefaultPriority string `toml:"default_priority"`
	} `toml:"tasks"`
	Search struct {
		MaxResults  int    `toml:"max_results"`
		SearchDepth string `toml:"search_depth"`
	} `toml:"search"`
}

var (
	cfg     Schema
	cfgOnce sync.Once
	cfgMu   sync.RWMutex
)

// Load reads the config file. If cfgFile is empty, uses the default path.
func Load(cfgFile string) error {
	var loadErr error
	cfgOnce.Do(func() {
		if cfgFile == "" {
			cfgFile = DefaultConfigPath()
		}
		loadErr = loadConfig(cfgFile)
	})
	return loadErr
}

func loadConfig(path string) error {
	cfgMu.Lock()
	defer cfgMu.Unlock()

	// Set defaults first
	cfg = defaultConfig()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Write defaults
		return writeConfig(path, cfg)
	}

	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	return nil
}

// Get returns a string config value by dot-path key.
func Get(key string) string {
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	switch key {
	case "general.default_model":
		return cfg.General.DefaultModel
	case "general.theme":
		return cfg.General.Theme
	case "general.default_provider":
		return cfg.General.DefaultProvider
	case "github.username":
		return cfg.GitHub.Username
	case "github.default_repo":
		return cfg.GitHub.DefaultRepo
	case "tasks.storage_path":
		return cfg.Tasks.StoragePath
	case "tasks.default_priority":
		return cfg.Tasks.DefaultPriority
	case "search.search_depth":
		return cfg.Search.SearchDepth
	case "ai.system_prompt":
		return cfg.AI.SystemPrompt
	}
	return ""
}

// GetBool returns a bool config value.
func GetBool(key string) bool {
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	switch key {
	case "general.auto_followup":
		return cfg.General.AutoFollowup
	case "general.stream":
		return cfg.General.Stream
	}
	return false
}

// GetInt returns an int config value.
func GetInt(key string) int {
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	switch key {
	case "general.history_limit":
		return cfg.General.HistoryLimit
	case "ai.max_tokens":
		return cfg.AI.MaxTokens
	case "search.max_results":
		return cfg.Search.MaxResults
	}
	return 0
}

// Set persists a config value by dot-path key.
func Set(key, value string) error {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	switch key {
	case "general.default_model":
		cfg.General.DefaultModel = value
	case "general.theme":
		cfg.General.Theme = value
	case "general.default_provider":
		cfg.General.DefaultProvider = value
	case "github.username":
		cfg.GitHub.Username = value
	case "github.default_repo":
		cfg.GitHub.DefaultRepo = value
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}
	return writeConfig(DefaultConfigPath(), cfg)
}

func writeConfig(path string, c Schema) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(c)
}

func defaultConfig() Schema {
	var c Schema
	c.General.DefaultModel = "llama-3.3-70b-versatile"
	c.General.Theme = "dark"
	c.General.HistoryLimit = 50
	c.General.AutoFollowup = true
	c.General.Stream = true
	c.AI.MaxTokens = 4096
	c.AI.Temperature = 0.7
	c.Tasks.StoragePath = filepath.Join(GooConfigDir(), "tasks.db")
	c.Tasks.DefaultPriority = "medium"
	c.Search.MaxResults = 5
	c.Search.SearchDepth = "basic"
	return c
}

// DefaultConfigPath returns ~/.config/goo/config.toml
func DefaultConfigPath() string {
	return filepath.Join(GooConfigDir(), "config.toml")
}

// GooConfigDir returns ~/.config/goo
func GooConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "goo")
}
