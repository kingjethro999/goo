package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

type Client struct {
	cfg       ServerConfig
	transport Transport
	mu        sync.RWMutex
	tools     []ToolDescriptor
	connected bool
}

func NewClient(cfg ServerConfig) (*Client, error) {
	var t Transport
	switch cfg.Transport {
	case TransportStdio:
		t = NewStdioTransport(cfg.Command, cfg.Args, cfg.Env)
	case TransportHTTP:
		t = NewHTTPTransport(cfg.URL, cfg.Headers)
	default:
		return nil, fmt.Errorf("mcp: unknown transport %q for server %q", cfg.Transport, cfg.Name)
	}
	return &Client{cfg: cfg, transport: t}, nil
}

func NewClientWithTransport(cfg ServerConfig, t Transport) *Client {
	return &Client{cfg: cfg, transport: t}
}

func initializeParams() map[string]any {
	return map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]string{
			"name":    "goo-cli",
			"version": "2.0.1",
		},
	}
}

func (c *Client) Connect(ctx context.Context) error {
	if err := c.transport.Connect(ctx); err != nil {
		return fmt.Errorf("mcp: connect %q: %w", c.cfg.Name, err)
	}
	if _, err := c.transport.Request(ctx, "initialize", initializeParams()); err != nil {
		return fmt.Errorf("mcp: initialize %q: %w", c.cfg.Name, err)
	}
	if err := c.transport.Notify(ctx, "notifications/initialized", nil); err != nil {
		return fmt.Errorf("mcp: initialized notify %q: %w", c.cfg.Name, err)
	}
	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()
	return nil
}

func (c *Client) ListTools(ctx context.Context) ([]ToolDescriptor, error) {
	raw, err := c.transport.Request(ctx, "tools/list", nil)
	if err != nil {
		return nil, fmt.Errorf("mcp: tools/list %q: %w", c.cfg.Name, err)
	}
	var resp struct {
		Tools []struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			InputSchema json.RawMessage `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("mcp: decode tools/list %q: %w", c.cfg.Name, err)
	}
	out := make([]ToolDescriptor, 0, len(resp.Tools))
	for _, t := range resp.Tools {
		out = append(out, ToolDescriptor{
			ServerName:  c.cfg.Name,
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}
	c.mu.Lock()
	c.tools = out
	c.mu.Unlock()
	return out, nil
}

func (c *Client) CallTool(ctx context.Context, name string, args json.RawMessage) (ToolResult, error) {
	raw, err := c.transport.Request(ctx, "tools/call", map[string]any{
		"name":      name,
		"arguments": json.RawMessage(args),
	})
	if err != nil {
		return ToolResult{}, fmt.Errorf("mcp: tools/call %s@%s: %w", name, c.cfg.Name, err)
	}
	var result ToolResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return ToolResult{}, fmt.Errorf("mcp: decode tools/call result: %w", err)
	}
	return result, nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()
	return c.transport.Close()
}

func (c *Client) Name() string {
	return c.cfg.Name
}

func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

func (c *Client) Tools() []ToolDescriptor {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]ToolDescriptor, len(c.tools))
	copy(out, c.tools)
	return out
}

func (c *Client) Config() ServerConfig {
	return c.cfg
}
