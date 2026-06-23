package core

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	findTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	findPathStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	findMatchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	findErrStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

type searchResult struct {
	path  string
	score int
	isDir bool
}

// RunFind executes an extensive deep search across the user's home directory.
func RunFind(query string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	fmt.Printf("%s\n\n", findTitleStyle.Render("🔍 Goo Find: Searching your system for '"+query+"'..."))

	// Extract query keywords (ignore words < 3 chars unless query is short)
	words := strings.Fields(strings.ToLower(query))
	var keywords []string
	for _, w := range words {
		w = strings.Trim(w, ".,!?'\"")
		if len(w) > 2 || len(words) == 1 {
			keywords = append(keywords, w)
		}
	}

	if len(keywords) == 0 {
		return fmt.Errorf("search query too short or vague")
	}

	var results []searchResult
	var scanned int
	startTime := time.Now()

	// Directories to explicitly ignore for performance
	ignoreDirs := map[string]bool{
		".git": true, "node_modules": true, "vendor": true,
		".cache": true, "Library": true, "AppData": true,
		".gemini": true, ".npm": true, ".rustup": true, ".cargo": true,
	}

	err = filepath.WalkDir(home, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // ignore permissions errors
		}

		if d.IsDir() {
			if ignoreDirs[d.Name()] {
				return filepath.SkipDir
			}
		}

		scanned++
		nameLower := strings.ToLower(d.Name())
		pathLower := strings.ToLower(path)

		score := 0
		matchesAll := true
		for _, kw := range keywords {
			if strings.Contains(nameLower, kw) {
				score += 10
				// Exact match bonus
				if nameLower == kw || strings.HasPrefix(nameLower, kw+".") {
					score += 20
				}
			} else if strings.Contains(pathLower, kw) {
				score += 2
			} else {
				matchesAll = false
			}
		}

		// Only include if it matched at least something strongly, or matched all keywords loosely
		if score > 0 && (matchesAll || score >= 10) {
			results = append(results, searchResult{
				path:  path,
				score: score,
				isDir: d.IsDir(),
			})
		}

		return nil
	})

	if err != nil {
		fmt.Println(findErrStyle.Render("Error during search: " + err.Error()))
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	duration := time.Since(startTime)
	fmt.Printf("Scanned %d items in %s.\n\n", scanned, duration.Round(time.Millisecond))

	if len(results) == 0 {
		fmt.Println(findErrStyle.Render("No matches found."))
		return nil
	}

	// Display top 10 results
	limit := 10
	if len(results) < limit {
		limit = len(results)
	}

	fmt.Printf("Top %d matches:\n", limit)
	for i := 0; i < limit; i++ {
		res := results[i]
		icon := "📄"
		if res.isDir {
			icon = "📁"
		}
		
		dir := filepath.Dir(res.path)
		base := filepath.Base(res.path)
		
		// Highlight keywords in the base name
		highlightedBase := base
		lowerBase := strings.ToLower(base)
		for _, kw := range keywords {
			if idx := strings.Index(lowerBase, kw); idx != -1 {
				original := base[idx : idx+len(kw)]
				highlightedBase = strings.ReplaceAll(highlightedBase, original, findMatchStyle.Render(original))
			}
		}

		fmt.Printf("%s %s/%s\n", icon, findPathStyle.Render(dir), highlightedBase)
	}

	return nil
}
