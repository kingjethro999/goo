package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kingjethro999/goo/internal/agent"
	"github.com/kingjethro999/goo/internal/tools"
)

type mockToolProvider struct{}

func (m *mockToolProvider) ToolsForScope(scope []string) []tools.Tool {
	return nil
}

func TestLoadTasksFile(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "tasks.yaml")
	content := `tasks:
  - id: t1
    description: "task 1"
    scope: ["file1.go"]
  - id: t2
    description: "task 2"
    scope: ["file2.go"]
`
	if err := os.WriteFile(yamlPath, []byte(content), 0644); err != nil {
		t.Fatalf("write temp yaml: %v", err)
	}

	tasks, err := LoadTasksFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadTasksFile failed: %v", err)
	}
	if len(tasks) != 2 || tasks[0].ID != "t1" || tasks[1].ID != "t2" {
		t.Errorf("unexpected tasks loaded: %+v", tasks)
	}
}

func TestMergeConflictsAndCleanDiffs(t *testing.T) {
	res1 := TaskResult{
		TaskID: "t1",
		Diffs: []agent.FileDiff{
			{Path: "common.go", OldContent: "a", NewContent: "b"},
			{Path: "file1.go", OldContent: "", NewContent: "1"},
		},
	}
	res2 := TaskResult{
		TaskID: "t2",
		Diffs: []agent.FileDiff{
			{Path: "common.go", OldContent: "a", NewContent: "c"},
			{Path: "file2.go", OldContent: "", NewContent: "2"},
		},
	}

	report := Merge([]TaskResult{res1, res2})
	if len(report.Conflicts) != 1 || report.Conflicts[0] != "common.go" {
		t.Errorf("expected conflict on common.go, got: %+v", report.Conflicts)
	}
	if len(report.CleanDiffs) != 2 {
		t.Errorf("expected 2 clean diffs, got: %+v", report.CleanDiffs)
	}
}

func TestRunParallelWithMockRunner(t *testing.T) {
	ctx := context.Background()
	lead := agent.NewCore(agent.CoreConfig{})
	policy := agent.NewStandardPolicy("allow", nil)
	orch := New(lead, 2, policy, &mockToolProvider{})

	orch.SetTestRunner(func(ctx context.Context, prompt string, c *agent.Core) ([]agent.FileDiff, []agent.Turn, error) {
		return []agent.FileDiff{
			{Path: prompt + ".go", OldContent: "", NewContent: "package main"},
		}, []agent.Turn{{Role: "assistant", Content: "done"}}, nil
	})

	tasks := []Task{
		{ID: "t1", Description: "task1", Scope: []string{"task1.go"}},
		{ID: "t2", Description: "task2", Scope: []string{"task2.go"}},
	}

	results := orch.RunParallel(ctx, tasks)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Err != nil {
			t.Errorf("unexpected task error: %v", r.Err)
		}
		if len(r.Diffs) != 1 {
			t.Errorf("expected 1 diff for task %s, got %d", r.TaskID, len(r.Diffs))
		}
	}
}
