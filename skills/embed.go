package skills

import (
	"embed"
	"io/fs"
	"strings"
)

//go:embed *.md
var BuiltinFS embed.FS

// GetBuiltinSkills returns a map of skill filename (e.g. "refactor.md") to its content
// for all built-in skills bundled with the Goo installation.
func GetBuiltinSkills() map[string]string {
	contents := make(map[string]string)

	entries, err := BuiltinFS.ReadDir(".")
	if err != nil {
		return contents
	}

	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "README.md" || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		data, err := fs.ReadFile(BuiltinFS, entry.Name())
		if err == nil {
			contents[entry.Name()] = string(data)
		}
	}
	return contents
}
