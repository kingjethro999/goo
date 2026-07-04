package core

import (
	"encoding/json"
	"testing"

	"github.com/kingjethro999/goo/tools/ai"
)

func TestIsPathInRoot(t *testing.T) {
	root := "/home/user/project"

	tests := []struct {
		path     string
		expected bool
	}{
		{"/home/user/project/file.txt", true},
		{"/home/user/project/sub/dir/file.txt", true},
		{"/home/user/project/../file.txt", false},
		{"/home/user/other/file.txt", false},
		{"/home/user/project", true},
	}

	for _, tc := range tests {
		res := isPathInRoot(tc.path, root)
		if res != tc.expected {
			t.Errorf("isPathInRoot(%q, %q) = %v; want %v", tc.path, root, res, tc.expected)
		}
	}
}

func TestClassifyShellCommand(t *testing.T) {
	root := "/home/user/project"

	tests := []struct {
		command  string
		cwd      string
		expected RiskTier
	}{
		{"ls", root, TierSafe},
		{"git status", root, TierSafe},
		{"npm install", root, TierLow},
		{"npm run dev", root, TierModerate},
		{"sudo apt update", root, TierHigh},
		{"rm -rf .git", root, TierHigh},
		{"git push origin main", root, TierHigh},
		{"ls", "/home/user", TierHigh}, // outside root
	}

	for _, tc := range tests {
		res := classifyShellCommand(tc.command, tc.cwd, root)
		if res != tc.expected {
			t.Errorf("classifyShellCommand(%q, %q, %q) = %v; want %v", tc.command, tc.cwd, root, res, tc.expected)
		}
	}
}

func TestClassifyToolCall(t *testing.T) {
	root := "/home/user/project"

	tests := []struct {
		name     string
		args     string
		expected RiskTier
	}{
		{"read_file", `{"path": "file.txt"}`, TierSafe},
		{"list_dir", `{"dir": "."}`, TierSafe},
		{"grep_search", `{"query": "foo"}`, TierSafe},
		{"write_file", `{"path": "file.txt"}`, TierLow},
		{"write_file", `{"path": "../file.txt"}`, TierHigh},
		{"run_command", `{"command": "npm install"}`, TierLow},
		{"run_command", `{"command": "sudo rm -rf /"}`, TierHigh},
	}

	for _, tc := range tests {
		call := &ai.ToolCall{
			Name:      tc.name,
			Arguments: json.RawMessage(tc.args),
		}
		res := classifyToolCall(call, root)
		if res != tc.expected {
			t.Errorf("classifyToolCall(%q, %q) = %v; want %v", tc.name, tc.args, res, tc.expected)
		}
	}
}

func TestLineDiff(t *testing.T) {
	oldStr := "hello\nworld\nfoo"
	newStr := "hello\nworld\nbar"

	diff := LineDiff(oldStr, newStr)
	// Output should contain - foo and + bar
	if !testing.Short() {
		t.Logf("Diff output:\n%s", diff)
	}
}
