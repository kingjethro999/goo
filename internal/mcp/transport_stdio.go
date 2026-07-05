package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
)

type StdioTransport struct {
	cmd     string
	args    []string
	env     map[string]string
	process *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser

	mu      sync.Mutex
	reqID   int
	pending map[int]chan RPCResponse
	closed  bool
}

func NewStdioTransport(command string, args []string, env map[string]string) *StdioTransport {
	return &StdioTransport{
		cmd:     command,
		args:    args,
		env:     env,
		pending: make(map[int]chan RPCResponse),
	}
}

func (t *StdioTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.process != nil && !t.closed {
		return nil
	}

	cmd := exec.CommandContext(ctx, t.cmd, t.args...)
	cmd.Env = os.Environ()
	for k, v := range t.env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, os.ExpandEnv(v)))
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdio stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdio stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("stdio start: %w", err)
	}

	t.process = cmd
	t.stdin = stdin
	t.stdout = stdout
	t.closed = false

	go t.readLoop(stdout)

	go func() {
		_ = cmd.Wait()
		t.mu.Lock()
		t.closed = true
		for _, ch := range t.pending {
			close(ch)
		}
		t.pending = make(map[int]chan RPCResponse)
		t.mu.Unlock()
	}()

	return nil
}

func (t *StdioTransport) readLoop(r io.Reader) {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024) // up to 10MB tokens

	for scanner.Scan() {
		line := scanner.Bytes()
		var resp RPCResponse
		if err := json.Unmarshal(line, &resp); err == nil && resp.ID > 0 {
			t.mu.Lock()
			ch, ok := t.pending[resp.ID]
			t.mu.Unlock()
			if ok {
				ch <- resp
			}
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "[mcp stdio transport]: read error: %v\n", err)
	}
}


func (t *StdioTransport) Request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	t.mu.Lock()
	if t.closed || t.process == nil {
		t.mu.Unlock()
		return nil, fmt.Errorf("stdio transport is closed or disconnected")
	}
	id := t.reqID + 1
	t.reqID = id
	ch := make(chan RPCResponse, 1)
	t.pending[id] = ch
	t.mu.Unlock()

	defer func() {
		t.mu.Lock()
		delete(t.pending, id)
		t.mu.Unlock()
	}()

	data, err := EncodeRequest(id, method, params)
	if err != nil {
		return nil, err
	}

	t.mu.Lock()
	_, writeErr := fmt.Fprintf(t.stdin, "%s\n", data)
	t.mu.Unlock()
	if writeErr != nil {
		return nil, fmt.Errorf("stdio write: %w", writeErr)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("stdio transport closed while waiting for response")
		}
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	}
}

func (t *StdioTransport) Notify(ctx context.Context, method string, params any) error {
	t.mu.Lock()
	if t.closed || t.process == nil {
		t.mu.Unlock()
		return fmt.Errorf("stdio transport is closed or disconnected")
	}
	t.mu.Unlock()

	data, err := EncodeNotify(method, params)
	if err != nil {
		return err
	}

	t.mu.Lock()
	_, writeErr := fmt.Fprintf(t.stdin, "%s\n", data)
	t.mu.Unlock()
	return writeErr
}

func (t *StdioTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil
	}
	t.closed = true
	if t.stdin != nil {
		_ = t.stdin.Close()
	}
	if t.process != nil && t.process.Process != nil {
		_ = t.process.Process.Kill()
	}
	for _, ch := range t.pending {
		close(ch)
	}
	t.pending = make(map[int]chan RPCResponse)
	return nil
}
