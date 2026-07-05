package orchestrator

import (
	"github.com/kingjethro999/goo/internal/agent"
	"github.com/kingjethro999/goo/internal/tools"
)

type Task struct {
	ID          string   `json:"id" yaml:"id"`
	Description string   `json:"description" yaml:"description"`
	Scope       []string `json:"scope" yaml:"scope"` // glob patterns this task may touch
	ParentID    string   `json:"parent_id,omitempty" yaml:"parent_id,omitempty"`
}

type TaskResult struct {
	TaskID string           `json:"task_id"`
	Diffs  []agent.FileDiff `json:"diffs"`
	Log    []agent.Turn     `json:"log"`
	Err    error            `json:"-"`
	ErrMsg string           `json:"err,omitempty"`
}

type ToolProvider interface {
	ToolsForScope(scope []string) []tools.Tool
}
