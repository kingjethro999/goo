package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewMCPHub(t *testing.T) {
	hub := NewMCPHub()
	if hub == nil {
		t.Fatal("expected NewMCPHub to return non-nil hub")
	}

	msg := hub.ListActiveServers()
	expected := "No active local MCP servers loaded."
	if msg != expected {
		t.Errorf("expected %q, got %q", expected, msg)
	}
}

func TestLoadAndInitialize_NoConfigFile(t *testing.T) {
	hub := NewMCPHub()
	tempDir := t.TempDir()

	tools, err := hub.LoadAndInitialize(tempDir)
	if err != nil {
		t.Fatalf("expected nil error when no mcp file exists, got: %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(tools))
	}
}

func TestLoadAndInitialize_InvalidJSON(t *testing.T) {
	hub := NewMCPHub()
	tempDir := t.TempDir()

	mcpPath := filepath.Join(tempDir, ".mcp.json")
	if err := os.WriteFile(mcpPath, []byte("invalid json content"), 0644); err != nil {
		t.Fatalf("failed to write dummy .mcp.json: %v", err)
	}

	_, err := hub.LoadAndInitialize(tempDir)
	if err == nil {
		t.Fatal("expected error when .mcp.json contains invalid json, got nil")
	}
}

func TestExecuteToolCall_NotFound(t *testing.T) {
	hub := NewMCPHub()
	_, err := hub.ExecuteToolCall("non_existent_tool", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for non-existent tool call, got nil")
	}
}
