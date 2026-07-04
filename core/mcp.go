package core

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kingjethro999/goo/config"
	"github.com/kingjethro999/goo/tools/ai"
)

type MCPServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}

type MCPConfigFile struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

type MCPServerProcess struct {
	Name    string
	Cmd     *exec.Cmd
	Stdin   io.WriteCloser
	Stdout  *bufio.Reader
	mu      sync.Mutex
	reqID   int
	Tools   []MCPTool
}

type MCPTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type MCPHub struct {
	mu        sync.RWMutex
	servers   map[string]*MCPServerProcess
	toolMap   map[string]*MCPServerProcess // toolName -> process
	origNames map[string]string            // mcp_server_tool -> origToolName
}

var GlobalMCPHub = NewMCPHub()

func NewMCPHub() *MCPHub {
	return &MCPHub{
		servers:   make(map[string]*MCPServerProcess),
		toolMap:   make(map[string]*MCPServerProcess),
		origNames: make(map[string]string),
	}
}

func (h *MCPHub) LoadAndInitialize(sessionRoot string) ([]ai.Tool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	var mcpFile string
	candidates := []string{
		filepath.Join(sessionRoot, ".mcp.json"),
		filepath.Join(sessionRoot, "goo.mcp.json"),
		filepath.Join(config.GooConfigDir(), "mcp.json"),
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			mcpFile = c
			break
		}
	}

	if mcpFile == "" {
		return nil, nil
	}

	data, err := os.ReadFile(mcpFile)
	if err != nil {
		return nil, err
	}

	var cfg MCPConfigFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid mcp config %s: %w", mcpFile, err)
	}

	var dynamicTools []ai.Tool

	for sName, sCfg := range cfg.MCPServers {
		proc, err := h.startServer(sName, sCfg)
		if err != nil {
			fmt.Printf("⚠️  Failed to start MCP server %s: %v\n", sName, err)
			continue
		}
		h.servers[sName] = proc

		for _, t := range proc.Tools {
			aiToolName := fmt.Sprintf("mcp_%s_%s", sName, t.Name)
			h.toolMap[aiToolName] = proc
			h.origNames[aiToolName] = t.Name

			schema := t.InputSchema
			if len(schema) == 0 {
				schema = json.RawMessage(`{"type":"object","properties":{}}`)
			}

			aiTool := ai.Tool{
				Type: "function",
				Function: &ai.ToolFunction{
					Name:        aiToolName,
					Description: fmt.Sprintf("[%s MCP] %s", sName, t.Description),
					Parameters:  schema,
				},
			}
			dynamicTools = append(dynamicTools, aiTool)
		}
	}

	return dynamicTools, nil
}

func (h *MCPHub) startServer(name string, cfg MCPServerConfig) (*MCPServerProcess, error) {
	cmd := exec.Command(cfg.Command, cfg.Args...)
	if len(cfg.Env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range cfg.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	proc := &MCPServerProcess{
		Name:   name,
		Cmd:    cmd,
		Stdin:  stdin,
		Stdout: bufio.NewReader(stdoutPipe),
	}

	// 1. Initialize request
	initReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      proc.nextID(),
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]string{
				"name":    "goo-cli",
				"version": "2.1.0",
			},
		},
	}
	if _, err := proc.sendRPC(initReq); err != nil {
		return nil, fmt.Errorf("init failed: %w", err)
	}

	// Send initialized notification
	notify := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	_ = proc.writeRPC(notify)

	// 2. List tools
	listReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      proc.nextID(),
		"method":  "tools/list",
	}
	respData, err := proc.sendRPC(listReq)
	if err != nil {
		return nil, fmt.Errorf("tools/list failed: %w", err)
	}

	var toolListResp struct {
		Result struct {
			Tools []MCPTool `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(respData, &toolListResp); err == nil {
		proc.Tools = toolListResp.Result.Tools
	}

	return proc, nil
}

func (p *MCPServerProcess) nextID() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.reqID++
	return p.reqID
}

func (p *MCPServerProcess) writeRPC(req interface{}) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := p.Stdin.Write([]byte(header + string(data))); err != nil {
		// Fallback without headers for standard JSON lines
		_, err = p.Stdin.Write(append(data, '\n'))
		return err
	}
	return nil
}

func (p *MCPServerProcess) sendRPC(req interface{}) ([]byte, error) {
	if err := p.writeRPC(req); err != nil {
		return nil, err
	}

	for {
		line, err := p.Stdout.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Content-Length:") {
			// Read remaining header
			_, _ = p.Stdout.ReadString('\n')
			// Read payload
			var contentLength int
			fmt.Sscanf(line, "Content-Length: %d", &contentLength)
			buf := make([]byte, contentLength)
			if _, err := io.ReadFull(p.Stdout, buf); err != nil {
				return nil, err
			}
			return buf, nil
		}
		if strings.HasPrefix(line, "{") {
			return []byte(line), nil
		}
	}
}

func (h *MCPHub) ExecuteToolCall(toolName string, arguments json.RawMessage) (string, error) {
	h.mu.RLock()
	proc, ok := h.toolMap[toolName]
	origName := h.origNames[toolName]
	h.mu.RUnlock()

	if !ok || proc == nil {
		return "", fmt.Errorf("MCP tool %s not found", toolName)
	}

	var argsObj interface{}
	if len(arguments) > 0 {
		_ = json.Unmarshal(arguments, &argsObj)
	}

	callReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      proc.nextID(),
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      origName,
			"arguments": argsObj,
		},
	}

	respBytes, err := proc.sendRPC(callReq)
	if err != nil {
		return "", fmt.Errorf("MCP execution error: %w", err)
	}

	var callResp struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBytes, &callResp); err != nil {
		return string(respBytes), nil
	}

	if callResp.Error != nil {
		return "", fmt.Errorf("MCP tool error: %s", callResp.Error.Message)
	}

	var sb strings.Builder
	for _, c := range callResp.Result.Content {
		sb.WriteString(c.Text)
		sb.WriteString("\n")
	}

	res := strings.TrimSpace(sb.String())
	if res == "" {
		res = "✓ MCP tool executed successfully."
	}
	return res, nil
}

func (h *MCPHub) ListActiveServers() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.servers) == 0 {
		return "No active local MCP servers loaded."
	}

	var sb strings.Builder
	sb.WriteString("Active Local MCP Servers:\n")
	for name, proc := range h.servers {
		sb.WriteString(fmt.Sprintf("  • %s (%d tools exposed):\n", name, len(proc.Tools)))
		for _, t := range proc.Tools {
			sb.WriteString(fmt.Sprintf("    - mcp_%s_%s: %s\n", name, t.Name, t.Description))
		}
	}
	return sb.String()
}
