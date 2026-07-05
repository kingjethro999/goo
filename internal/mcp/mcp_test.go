package mcp

import (
	"context"
	"encoding/json"
	"testing"
)

type mockTransport struct {
	connected bool
	reqs      []string
}

func (m *mockTransport) Connect(ctx context.Context) error {
	m.connected = true
	return nil
}

func (m *mockTransport) Request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	m.reqs = append(m.reqs, method)
	switch method {
	case "initialize":
		return json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"serverInfo":{"name":"mock","version":"1.0"}}`), nil
	case "tools/list":
		return json.RawMessage(`{"tools":[{"name":"read_foo","description":"reads foo","inputSchema":{"type":"object"}}]}`), nil
	case "tools/call":
		return json.RawMessage(`{"content":[{"type":"text","text":"foo content"}],"isError":false}`), nil
	default:
		return nil, nil
	}
}

func (m *mockTransport) Notify(ctx context.Context, method string, params any) error {
	m.reqs = append(m.reqs, method)
	return nil
}

func (m *mockTransport) Close() error {
	m.connected = false
	return nil
}

func TestClientRoundTrip(t *testing.T) {
	ctx := context.Background()
	mock := &mockTransport{}
	client := NewClientWithTransport(ServerConfig{Name: "test-server", Transport: TransportStdio}, mock)

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	if !client.IsConnected() {
		t.Errorf("expected client to be connected")
	}

	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "read_foo" {
		t.Errorf("unexpected tools: %+v", tools)
	}

	res, err := client.CallTool(ctx, "read_foo", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}
	if res.IsError || len(res.Content) != 1 || res.Content[0].Text != "foo content" {
		t.Errorf("unexpected result: %+v", res)
	}

	if err := client.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if client.IsConnected() {
		t.Errorf("expected client to be disconnected")
	}
}
