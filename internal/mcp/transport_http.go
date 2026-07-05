package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

type HTTPTransport struct {
	url     string
	headers map[string]string
	client  *http.Client
	mu      sync.Mutex
	reqID   int
	closed  bool
}

func NewHTTPTransport(url string, headers map[string]string) *HTTPTransport {
	expanded := make(map[string]string, len(headers))
	for k, v := range headers {
		expanded[k] = os.ExpandEnv(v)
	}
	return &HTTPTransport{
		url:     url,
		headers: expanded,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (t *HTTPTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = false
	return nil
}


func (t *HTTPTransport) Request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil, fmt.Errorf("http transport is closed")
	}
	id := t.reqID + 1
	t.reqID = id
	t.mu.Unlock()

	data, err := EncodeRequest(id, method, params)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("http new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("http read body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("http status %d: %s", resp.StatusCode, string(body))
	}

	var rpcResp RPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return nil, fmt.Errorf("http decode rpc response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, rpcResp.Error
	}
	return rpcResp.Result, nil
}

func (t *HTTPTransport) Notify(ctx context.Context, method string, params any) error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return fmt.Errorf("http transport is closed")
	}
	t.mu.Unlock()

	data, err := EncodeNotify(method, params)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("http notify request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("http notify failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("http notify status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (t *HTTPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = true
	return nil
}
