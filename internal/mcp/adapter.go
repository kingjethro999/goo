package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kingjethro999/goo/internal/agent"
	"github.com/kingjethro999/goo/internal/tools"
)

type toolAdapter struct {
	client *Client
	desc   ToolDescriptor
	policy agent.SafetyPolicy
}

func (a *toolAdapter) Name() string {
	return fmt.Sprintf("%s:%s", a.desc.ServerName, a.desc.Name)
}

func (a *toolAdapter) Description() string {
	return a.desc.Description
}

func (a *toolAdapter) Schema() tools.ToolSchema {
	return tools.ToolSchema{Raw: a.desc.InputSchema}
}

func (a *toolAdapter) Execute(ctx context.Context, input json.RawMessage) (tools.ToolResult, error) {
	decision := a.policy.Check(agent.Action{
		Kind:   agent.ActionMCPCall,
		Target: a.Name(),
		Detail: string(input),
	})
	if decision == agent.Deny {
		return tools.ToolResult{}, fmt.Errorf("blocked by safety policy: %s", a.Name())
	}
	if decision == agent.Ask {
		if !agent.Confirm(ctx, a.Name(), string(input)) {
			return tools.ToolResult{}, fmt.Errorf("declined by user: %s", a.Name())
		}
	}
	res, err := a.client.CallTool(ctx, a.desc.Name, input)
	if err != nil && !a.client.IsConnected() {
		// Attempt one reconnect before surfacing the error to the model
		_ = a.client.Close()
		if reconnErr := a.client.Connect(ctx); reconnErr == nil {
			res, err = a.client.CallTool(ctx, a.desc.Name, input)
		}
	}
	if err != nil {
		return tools.ToolResult{}, err
	}
	return tools.ToolResult{Content: mcpContentToNative(res.Content), IsError: res.IsError}, nil
}

func mcpContentToNative(blocks []ContentBlock) []tools.ContentBlock {
	out := make([]tools.ContentBlock, len(blocks))
	for i, b := range blocks {
		out[i] = tools.ContentBlock{Type: b.Type, Text: b.Text, Data: b.Data}
	}
	return out
}

func BuildToolList(mgr *Manager, servers []string, policy agent.SafetyPolicy) []tools.Tool {
	var out []tools.Tool
	if len(servers) == 0 {
		for _, client := range mgr.Clients() {
			for _, d := range client.Tools() {
				out = append(out, &toolAdapter{client: client, desc: d, policy: policy})
			}
		}
		return out
	}
	for _, name := range servers {
		client, ok := mgr.Client(name)
		if !ok {
			continue
		}
		for _, d := range client.Tools() {
			out = append(out, &toolAdapter{client: client, desc: d, policy: policy})
		}
	}
	return out
}
