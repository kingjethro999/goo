package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kingjethro999/goo/skills"
)

// Skill represents a modular, reusable agent instruction recipe or slash command workflow.
type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Trigger     string `json:"trigger"` // e.g. "/test-fix"
	Path        string `json:"path"`
	Prompt      string `json:"prompt"`
}

// LoadSkills scans both local project skills (.goo/skills, .claude/skills) and global user skills (~/.config/goo/skills).
func LoadSkills(projectRoot string) ([]Skill, error) {
	var skillsList []Skill
	seen := make(map[string]bool)

	// Directories to check for skills
	searchDirs := []string{}

	if projectRoot != "" {
		searchDirs = append(searchDirs,
			filepath.Join(projectRoot, ".goo", "skills"),
			filepath.Join(projectRoot, ".claude", "skills"),
			filepath.Join(projectRoot, ".cursor", "skills"),
		)
	}

	home, err := os.UserHomeDir()
	if err == nil {
		globalSkillsDir := filepath.Join(home, ".config", "goo", "skills")
		_ = os.MkdirAll(globalSkillsDir, 0755)
		// Auto-install built-in skills to global user directory if not already present
		for fileName, content := range skills.GetBuiltinSkills() {
			targetPath := filepath.Join(globalSkillsDir, fileName)
			if _, err := os.Stat(targetPath); os.IsNotExist(err) {
				_ = os.WriteFile(targetPath, []byte(content), 0644)
			}
		}
		searchDirs = append(searchDirs, globalSkillsDir)
	}

	for _, dir := range searchDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}

			filePath := filepath.Join(dir, entry.Name())
			contentBytes, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			content := string(contentBytes)
			skillName := strings.TrimSuffix(entry.Name(), ".md")
			if seen[skillName] {
				continue
			}

			desc, trigger, body := parseSkillMarkdown(skillName, content)
			skillsList = append(skillsList, Skill{
				Name:        skillName,
				Description: desc,
				Trigger:     trigger,
				Path:        filePath,
				Prompt:      body,
			})
			seen[skillName] = true
		}
	}

	// Fallback load directly from embedded builtin skills if not loaded from files
	for fileName, content := range skills.GetBuiltinSkills() {
		skillName := strings.TrimSuffix(fileName, ".md")
		if seen[skillName] {
			continue
		}
		desc, trigger, body := parseSkillMarkdown(skillName, content)
		skillsList = append(skillsList, Skill{
			Name:        skillName,
			Description: desc,
			Trigger:     trigger,
			Path:        "builtin://" + fileName,
			Prompt:      body,
		})
		seen[skillName] = true
	}

	return skillsList, nil
}

// parseSkillMarkdown extracts frontmatter or metadata headers from skill markdown files.
func parseSkillMarkdown(defaultName, content string) (desc, trigger, body string) {
	lines := strings.Split(content, "\n")
	var promptLines []string
	desc = fmt.Sprintf("Custom skill: %s", defaultName)
	trigger = "/" + defaultName

	inFrontmatter := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if i == 0 && trimmed == "---" {
			inFrontmatter = true
			continue
		}
		if inFrontmatter {
			if trimmed == "---" {
				inFrontmatter = false
				continue
			}
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 {
				key := strings.ToLower(strings.TrimSpace(parts[0]))
				val := strings.TrimSpace(parts[1])
				switch key {
				case "description":
					desc = val
				case "trigger", "command":
					if !strings.HasPrefix(val, "/") {
						val = "/" + val
					}
					trigger = val
				}
			}
			continue
		}
		promptLines = append(promptLines, line)
	}

	body = strings.TrimSpace(strings.Join(promptLines, "\n"))
	return desc, trigger, body
}

// BuildSkillsPromptBlock formats available skills into an prompt section for the AI agent.
func BuildSkillsPromptBlock(skills []Skill) string {
	if len(skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Available Agent Skills:\n")
	for _, s := range skills {
		sb.WriteString(fmt.Sprintf("- Trigger: %s | Name: %s | Description: %s\n  Prompt Recipe:\n%s\n\n",
			s.Trigger, s.Name, s.Description, indentBlock(s.Prompt, "    ")))
	}
	return sb.String()
}

func indentBlock(s, indent string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = indent + l
	}
	return strings.Join(lines, "\n")
}
