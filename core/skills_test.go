package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSkillMarkdown(t *testing.T) {
	content := `---
description: Run go test and fix any failing test cases automatically
trigger: /test-and-fix
---
Please run 'go test ./...' and if any test fails, analyze the test failure log and update the source code to fix it.`

	desc, trigger, body := parseSkillMarkdown("test-fix", content)

	if desc != "Run go test and fix any failing test cases automatically" {
		t.Errorf("expected description match, got %q", desc)
	}

	if trigger != "/test-and-fix" {
		t.Errorf("expected trigger '/test-and-fix', got %q", trigger)
	}

	if body == "" {
		t.Error("expected non-empty skill prompt body")
	}
}

func TestLoadSkills(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "goo_skills_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	skillsDir := filepath.Join(tempDir, ".goo", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("failed to create skills dir: %v", err)
	}

	sampleSkill := `---
description: Auto refactor function
trigger: /refactor
---
Refactor the target function into clean reusable modules.`

	if err := os.WriteFile(filepath.Join(skillsDir, "refactor.md"), []byte(sampleSkill), 0644); err != nil {
		t.Fatalf("failed to write skill file: %v", err)
	}

	skills, err := LoadSkills(tempDir)
	if err != nil {
		t.Fatalf("failed to load skills: %v", err)
	}

	if len(skills) == 0 {
		t.Fatalf("expected at least 1 skill, got 0")
	}

	if skills[0].Name != "refactor" {
		t.Errorf("expected skill name 'refactor', got %q", skills[0].Name)
	}

	promptBlock := BuildSkillsPromptBlock(skills)
	if promptBlock == "" {
		t.Error("expected non-empty skills prompt block")
	}
}
