package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
)

type Manager struct {
	mu      sync.RWMutex
	clients map[string]*Client
	logger  *slog.Logger
}

func NewManager(logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	return &Manager{clients: make(map[string]*Client), logger: logger}
}

func (m *Manager) LoadFromConfig(ctx context.Context, servers []ServerConfig) error {
	for _, sc := range servers {
		if !sc.Enabled {
			continue
		}
		client, err := NewClient(sc)
		if err != nil {
			m.logger.Warn("mcp client creation failed, skipping", "server", sc.Name, "error", err)
			continue
		}
		if err := client.Connect(ctx); err != nil {
			m.logger.Warn("mcp server failed to connect, skipping", "server", sc.Name, "error", err)
			continue
		}
		if _, err := client.ListTools(ctx); err != nil {
			m.logger.Warn("mcp server connected but tools/list failed", "server", sc.Name, "error", err)
			continue
		}
		m.mu.Lock()
		m.clients[sc.Name] = client
		m.mu.Unlock()
	}
	return nil
}

func (m *Manager) Client(name string) (*Client, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.clients[name]
	return c, ok
}

func (m *Manager) Clients() map[string]*Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]*Client, len(m.clients))
	for k, v := range m.clients {
		out[k] = v
	}
	return out
}

func (m *Manager) AddClient(c *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[c.Name()] = c
}

func (m *Manager) RemoveClient(name string) error {
	m.mu.Lock()
	c, ok := m.clients[name]
	if ok {
		delete(m.clients, name)
	}
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("mcp: no server named %q", name)
	}
	return c.Close()
}

func (m *Manager) Reconnect(ctx context.Context, name string) error {
	m.mu.RLock()
	c, ok := m.clients[name]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("mcp: no server named %q", name)
	}
	_ = c.Close()
	return c.Connect(ctx)
}

func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.clients {
		_ = c.Close()
	}
	m.clients = make(map[string]*Client)
}
