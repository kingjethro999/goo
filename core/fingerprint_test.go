package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateFingerprint_NodeNext(t *testing.T) {
	tempDir := t.TempDir()

	// Create a package.json indicating Next.js
	packageJSON := `{
  "name": "my-app",
  "dependencies": {
    "next": "14.0.0",
    "react": "18.2.0"
  }
}`
	if err := os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	// Create some folder structure
	os.MkdirAll(filepath.Join(tempDir, "src", "components"), 0755)
	os.MkdirAll(filepath.Join(tempDir, "node_modules", "next"), 0755) // Should be skipped
	os.WriteFile(filepath.Join(tempDir, "src", "index.js"), []byte("console.log('hi');"), 0644)

	fp, err := GenerateFingerprint(tempDir)
	if err != nil {
		t.Fatalf("GenerateFingerprint failed: %v", err)
	}

	if fp.ProjectType != "NodeJS" {
		t.Errorf("expected ProjectType 'NodeJS', got %q", fp.ProjectType)
	}
	if fp.Framework != "Next.js" {
		t.Errorf("expected Framework 'Next.js', got %q", fp.Framework)
	}

	// Ensure node_modules was skipped
	for _, p := range fp.Structure {
		if filepath.Base(p) == "node_modules" {
			t.Errorf("expected node_modules to be skipped in structure, but found %q", p)
		}
	}
}

func TestGenerateFingerprint_GoProject(t *testing.T) {
	tempDir := t.TempDir()

	// Create a go.mod file
	goMod := "module github.com/example/demo\n\ngo 1.22\n"
	if err := os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	fp, err := GenerateFingerprint(tempDir)
	if err != nil {
		t.Fatalf("GenerateFingerprint failed: %v", err)
	}

	if fp.ProjectType != "Go" {
		t.Errorf("expected ProjectType 'Go', got %q", fp.ProjectType)
	}
	if fp.Framework != "" {
		t.Errorf("expected empty Framework for Go project, got %q", fp.Framework)
	}
}
