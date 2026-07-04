package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kingjethro999/goo/tools/ai"
)

func TestExecuteToolCall_ListDirAndGrep(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "test1.txt")
	file2 := filepath.Join(tmpDir, "test2.go")

	if err := os.WriteFile(file1, []byte("hello world\nfoo bar\n"), 0644); err != nil {
		t.Fatalf("failed to create test1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("package main\nfunc main() {\n\tprintln(\"hello goo\")\n}\n"), 0644); err != nil {
		t.Fatalf("failed to create test2: %v", err)
	}

	deps := ToolDeps{}

	// Test list_dir
	callList := &ai.ToolCall{
		Name:      "list_dir",
		Arguments: json.RawMessage(`{"dir": "` + tmpDir + `"}`),
	}
	out, err := ExecuteToolCall(callList, deps)
	if err != nil {
		t.Fatalf("list_dir failed: %v", err)
	}
	if !strings.Contains(out, "test1.txt") || !strings.Contains(out, "test2.go") {
		t.Errorf("list_dir output missing files: %s", out)
	}

	// Test grep_search
	callGrep := &ai.ToolCall{
		Name:      "grep_search",
		Arguments: json.RawMessage(`{"query": "hello", "path": "` + tmpDir + `"}`),
	}
	out, err = ExecuteToolCall(callGrep, deps)
	if err != nil {
		t.Fatalf("grep_search failed: %v", err)
	}
	if !strings.Contains(out, "hello world") || !strings.Contains(out, "hello goo") {
		t.Errorf("grep_search output missing matches: %s", out)
	}
}

func TestExecuteToolCall_ReadFileRange(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "lines.txt")
	content := "line 1\nline 2\nline 3\nline 4\nline 5"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write lines.txt: %v", err)
	}

	deps := ToolDeps{}
	callRead := &ai.ToolCall{
		Name:      "read_file",
		Arguments: json.RawMessage(`{"path": "` + filePath + `", "start_line": 2, "end_line": 4}`),
	}
	out, err := ExecuteToolCall(callRead, deps)
	if err != nil {
		t.Fatalf("read_file range failed: %v", err)
	}
	if !strings.Contains(out, "2: line 2") || !strings.Contains(out, "4: line 4") || strings.Contains(out, "line 1") || strings.Contains(out, "line 5") {
		t.Errorf("read_file range unexpected output:\n%s", out)
	}
}

func TestExecuteToolCall_WriteFileRangeAndDiag(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "code.txt")
	content := "func alpha() {}\nfunc beta() {\n\treturn\n}\nfunc gamma() {}\nfunc beta() {\n\treturn 2\n}"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write code.txt: %v", err)
	}

	deps := ToolDeps{}

	// Test diagnostic error when exact match fails due to whitespace
	callDiag := &ai.ToolCall{
		Name:      "write_file",
		Arguments: json.RawMessage(`{"path": "` + filePath + `", "old_content": "func alpha() {   }", "new_content": "func alpha() { println() }"}`),
	}
	_, err := ExecuteToolCall(callDiag, deps)
	if err == nil {
		t.Fatalf("expected diagnostic error for whitespace mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "looks similar") && !strings.Contains(err.Error(), "whitespace") {
		t.Errorf("expected helpful diagnostic message, got: %v", err)
	}

	// Test scoped replacement when old_content appears multiple times
	callScoped := &ai.ToolCall{
		Name:      "write_file",
		Arguments: json.RawMessage(`{"path": "` + filePath + `", "old_content": "func beta() {\n\treturn 2\n}", "new_content": "func beta() {\n\treturn 3\n}", "start_line": 5, "end_line": 8}`),
	}
	out, err := ExecuteToolCall(callScoped, deps)
	if err != nil {
		t.Fatalf("scoped write_file failed: %v", err)
	}
	if !strings.Contains(out, "Successfully updated") {
		t.Errorf("unexpected output: %s", out)
	}

	newBytes, _ := os.ReadFile(filePath)
	newStr := string(newBytes)
	if !strings.Contains(newStr, "return 3") || !strings.Contains(newStr, "return\n") {
		t.Errorf("scoped replacement failed, file content:\n%s", newStr)
	}
}
