package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type ProjectFingerprint struct {
	ProjectType string   `json:"project_type,omitempty"`
	Framework   string   `json:"framework,omitempty"`
	Structure   []string `json:"structure,omitempty"`
}

func GenerateFingerprint(dir string) (*ProjectFingerprint, error) {
	fp := &ProjectFingerprint{}
	
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	
	var detectedTypes []string
	for _, f := range files {
		name := f.Name()
		switch name {
		case "package.json":
			detectedTypes = append(detectedTypes, "NodeJS")
			if data, err := os.ReadFile(filepath.Join(dir, "package.json")); err == nil {
				var pjson struct {
					Dependencies    map[string]string `json:"dependencies"`
					DevDependencies map[string]string `json:"devDependencies"`
				}
				if json.Unmarshal(data, &pjson) == nil {
					if _, ok := pjson.Dependencies["next"]; ok {
						fp.Framework = "Next.js"
					} else if _, ok := pjson.Dependencies["express"]; ok {
						fp.Framework = "Express"
					} else if _, ok := pjson.Dependencies["react"]; ok {
						fp.Framework = "React"
					}
				}
			}
		case "go.mod":
			detectedTypes = append(detectedTypes, "Go")
		case "Cargo.toml":
			detectedTypes = append(detectedTypes, "Rust")
		case "requirements.txt", "pyproject.toml":
			detectedTypes = append(detectedTypes, "Python")
		}
	}
	
	if len(detectedTypes) > 0 {
		fp.ProjectType = strings.Join(detectedTypes, ", ")
	} else {
		fp.ProjectType = "Unknown/Empty"
	}
	
	var structure []string
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil || rel == "." {
			return nil
		}
		
		parts := strings.Split(rel, string(os.PathSeparator))
		for _, part := range parts {
			if part == "node_modules" || part == ".git" || part == "dist" || part == "build" || part == "bin" {
				return filepath.SkipDir
			}
		}
		
		if len(parts) <= 2 {
			structure = append(structure, rel)
		} else {
			return filepath.SkipDir
		}
		return nil
	})
	fp.Structure = structure
	return fp, nil
}
