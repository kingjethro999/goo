package mcp

import (
	"context"
	"encoding/json"
)

type TransportKind string

const (
	TransportStdio TransportKind = "stdio"
	TransportHTTP  TransportKind = "http"
)

type ServerConfig struct {
	Name         string            `json:"name" toml:"name" yaml:"name"`
	Transport    TransportKind     `json:"transport" toml:"transport" yaml:"transport"`
	Command      string            `json:"command,omitempty" toml:"command,omitempty" yaml:"command,omitempty"`
	Args         []string          `json:"args,omitempty" toml:"args,omitempty" yaml:"args,omitempty"`
	Env          map[string]string `json:"env,omitempty" toml:"env,omitempty" yaml:"env,omitempty"`
	URL          string            `json:"url,omitempty" toml:"url,omitempty" yaml:"url,omitempty"`
	Headers      map[string]string `json:"headers,omitempty" toml:"headers,omitempty" yaml:"headers,omitempty"`
	Enabled      bool              `json:"enabled" toml:"enabled" yaml:"enabled"`
	SafetyPolicy string            `json:"safety_policy,omitempty" toml:"safety_policy,omitempty" yaml:"safety_policy,omitempty"`
}

type ToolDescriptor struct {
	ServerName  string          `json:"serverName"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type ContentBlock struct {
	Type string `json:"type"` // "text", "image", "resource"
	Text string `json:"text,omitempty"`
	Data string `json:"data,omitempty"`
}

type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError"`
}

type Transport interface {
	Connect(ctx context.Context) error
	Request(ctx context.Context, method string, params any) (json.RawMessage, error)
	Notify(ctx context.Context, method string, params any) error
	Close() error
}
