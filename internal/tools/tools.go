package tools

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/kingjethro999/goo/tools/ai"
)

type ToolSchema struct {
	Raw json.RawMessage
}

type ContentBlock struct {
	Type string // "text", "image", "resource"
	Text string
	Data string // base64, for non-text blocks
}

type ToolResult struct {
	Content []ContentBlock
	IsError bool
}

type Tool interface {
	Name() string
	Description() string
	Schema() ToolSchema
	Execute(ctx context.Context, input json.RawMessage) (ToolResult, error)
}

type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
	order []Tool
}

func NewRegistry(toolsList ...[]Tool) *Registry {
	r := &Registry{
		tools: make(map[string]Tool),
	}
	for _, list := range toolsList {
		for _, t := range list {
			r.Register(t)
		}
	}
	return r
}

func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tools[t.Name()]; !ok {
		r.order = append(r.order, t)
	} else {
		for i, existing := range r.order {
			if existing.Name() == t.Name() {
				r.order[i] = t
				break
			}
		}
	}
	r.tools[t.Name()] = t
}

func (r *Registry) All() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Tool, len(r.order))
	copy(out, r.order)
	return out
}

func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) ToAITools() []ai.Tool {
	all := r.All()
	out := make([]ai.Tool, 0, len(all))
	for _, t := range all {
		schema := t.Schema().Raw
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		out = append(out, ai.Tool{
			Type: "function",
			Function: &ai.ToolFunction{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  schema,
			},
		})
	}
	return out
}
