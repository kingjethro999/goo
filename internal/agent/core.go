package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kingjethro999/goo/config"
	"github.com/kingjethro999/goo/internal/tools"
	"github.com/kingjethro999/goo/memory"
	"github.com/kingjethro999/goo/tools/ai"
)

type FileDiff struct {
	Path       string `json:"path"`
	OldContent string `json:"old_content"`
	NewContent string `json:"new_content"`
}

type Turn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type CoreConfig struct {
	Tools      []tools.Tool
	Policy     SafetyPolicy
	TestRunner func(ctx context.Context, prompt string, c *Core) ([]FileDiff, []Turn, error)
}

type Core struct {
	cfg CoreConfig
}

func NewCore(cfg CoreConfig) *Core {
	if cfg.Policy == nil {
		cfg.Policy = NewStandardPolicy("ask", nil)
	}
	return &Core{cfg: cfg}
}

func (c *Core) Policy() SafetyPolicy {
	return c.cfg.Policy
}

func (c *Core) Tools() []tools.Tool {
	return c.cfg.Tools
}

func (c *Core) RunToCompletion(ctx context.Context, prompt string) ([]FileDiff, []Turn, error) {
	if c.cfg.TestRunner != nil {
		return c.cfg.TestRunner(ctx, prompt, c)
	}

	var diffs []FileDiff
	var turns []Turn

	turns = append(turns, Turn{Role: "user", Content: prompt})

	reg := tools.NewRegistry(c.cfg.Tools)
	aiTools := reg.ToAITools()

	client, err := ai.NewGroqClient()
	if err != nil {
		return nil, turns, fmt.Errorf("agent: failed to create AI client: %w", err)
	}

	model := config.Get("general.default_model")
	if model == "" {
		model = "openai/gpt-oss-120b"
	}

	messages := []memory.Message{
		{Role: "system", Content: "You are a sub-agent executing a delegated task. Use available tools to accomplish the task concisely."},
		{Role: "user", Content: prompt},
	}

	for step := 0; step < 10; step++ {
		select {
		case <-ctx.Done():
			return diffs, turns, ctx.Err()
		default:
		}

		var respBuf strings.Builder
		opts := ai.StreamOptions{
			Model: model,
		}

		toolCall, err := client.StreamChatWithToolsEx(ctx, messages, &respBuf, aiTools, opts)
		if err != nil {
			return diffs, turns, fmt.Errorf("agent AI call failed: %w", err)
		}

		content := respBuf.String()
		if content != "" {
			turns = append(turns, Turn{Role: "assistant", Content: content})
			messages = append(messages, memory.Message{Role: "assistant", Content: content})
		}

		if toolCall == nil {
			break
		}

		call := toolCall
		var kind ActionKind = ActionMCPCall
		var target string = call.Name

		if call.Name == "write_file" || call.Name == "file_edit" || strings.HasSuffix(call.Name, ":write_file") {
			kind = ActionFileWrite
			var args struct {
				Path string `json:"path"`
			}
			_ = json.Unmarshal(call.Arguments, &args)
			target = args.Path
		} else if call.Name == "run_command" || call.Name == "exec" || strings.HasSuffix(call.Name, ":run_command") {
			kind = ActionShellExec
			var args struct {
				Command string `json:"command"`
			}
			_ = json.Unmarshal(call.Arguments, &args)
			target = args.Command
		}

		decision := c.cfg.Policy.Check(Action{
			Kind:   kind,
			Target: target,
			Detail: string(call.Arguments),
		})

		if decision == Deny {
			errMsg := fmt.Sprintf("Error: blocked by safety policy for target %q", target)
			turns = append(turns, Turn{Role: "tool", Content: errMsg})
			messages = append(messages, memory.Message{Role: "tool", Content: errMsg, ToolCallID: call.ID, ToolName: call.Name})
			continue
		}

		if decision == Ask {
			if !Confirm(ctx, target, string(call.Arguments)) {
				errMsg := fmt.Sprintf("Error: declined by user for target %q", target)
				turns = append(turns, Turn{Role: "tool", Content: errMsg})
				messages = append(messages, memory.Message{Role: "tool", Content: errMsg, ToolCallID: call.ID, ToolName: call.Name})
				continue
			}
		}

		t, ok := reg.Get(call.Name)
		if !ok {
			errMsg := fmt.Sprintf("Error: tool %s not found in sub-agent scope", call.Name)
			turns = append(turns, Turn{Role: "tool", Content: errMsg})
			messages = append(messages, memory.Message{Role: "tool", Content: errMsg, ToolCallID: call.ID, ToolName: call.Name})
			continue
		}

		res, err := t.Execute(ctx, call.Arguments)
		if err != nil {
			turns = append(turns, Turn{Role: "tool", Content: err.Error()})
			messages = append(messages, memory.Message{Role: "tool", Content: err.Error(), ToolCallID: call.ID, ToolName: call.Name})
			continue
		}

		if kind == ActionFileWrite && !res.IsError {
			var writeArgs struct {
				Path       string `json:"path"`
				OldContent string `json:"old_content"`
				NewContent string `json:"new_content"`
				Content    string `json:"content"`
			}
			if err := json.Unmarshal(call.Arguments, &writeArgs); err == nil {
				newC := writeArgs.NewContent
				if newC == "" {
					newC = writeArgs.Content
				}
				diffs = append(diffs, FileDiff{
					Path:       writeArgs.Path,
					OldContent: writeArgs.OldContent,
					NewContent: newC,
				})
			}
		}

		var resText strings.Builder
		for _, b := range res.Content {
			if b.Text != "" {
				resText.WriteString(b.Text)
			}
		}
		turns = append(turns, Turn{Role: "tool", Content: resText.String()})
		messages = append(messages, memory.Message{Role: "tool", Content: resText.String(), ToolCallID: call.ID, ToolName: call.Name})
	}

	return diffs, turns, nil
}
