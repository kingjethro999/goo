package agent

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type ActionKind string

const (
	ActionFileWrite ActionKind = "file_write"
	ActionShellExec ActionKind = "shell_exec"
	ActionMCPCall   ActionKind = "mcp_call"
)

type Action struct {
	Kind   ActionKind
	Target string // file path, shell command, or server:tool
	Detail string // file content diff snippet, full args, or JSON payload
}

type Decision string

const (
	Allow Decision = "allow"
	Deny  Decision = "deny"
	Ask   Decision = "ask"
)

type SafetyPolicy interface {
	Check(action Action) Decision
	Narrow(scope []string) SafetyPolicy
}

type StandardPolicy struct {
	Mode      string   // "allow", "ask", "deny", "always_confirm", "allowlist", "diff_approve"
	Scope     []string // glob patterns allowed. If empty/nil, global scope.
	Allowlist []string // commands or tools auto-allowed
}

func NewStandardPolicy(mode string, allowlist []string) *StandardPolicy {
	if mode == "" {
		mode = "ask"
	}
	return &StandardPolicy{
		Mode:      mode,
		Allowlist: allowlist,
	}
}

func (p *StandardPolicy) Check(action Action) Decision {
	if p.Mode == "deny" {
		return Deny
	}

	// Check scope restrictions if narrowed (e.g. sub-agent)
	if len(p.Scope) > 0 {
		if action.Kind == ActionFileWrite {
			inScope := false
			targetPath := action.Target
			for _, pattern := range p.Scope {
				matched, _ := filepath.Match(pattern, targetPath)
				if !matched {
					matched, _ := filepath.Match(pattern, filepath.Base(targetPath))
					if matched {
						inScope = true
						break
					}
				} else {
					inScope = true
					break
				}
				if strings.Contains(targetPath, strings.TrimPrefix(pattern, "./")) {
					inScope = true
					break
				}
			}
			if !inScope {
				return Deny
			}
		} else if action.Kind == ActionShellExec {
			// In narrowed scope, shell exec is denied by default unless explicitly allowlisted
			if !p.isAllowlisted(action.Target) {
				return Deny
			}
		}
	}

	if p.Mode == "allow" {
		return Allow
	}

	if p.Mode == "allowlist" && p.isAllowlisted(action.Target) {
		return Allow
	}

	if p.Mode == "ask" || p.Mode == "always_confirm" || p.Mode == "diff_approve" {
		return Ask
	}

	return Ask
}

func (p *StandardPolicy) isAllowlisted(target string) bool {
	for _, item := range p.Allowlist {
		if item == target || strings.HasPrefix(target, strings.TrimSuffix(item, "*")) {
			return true
		}
	}
	return false
}

func (p *StandardPolicy) Narrow(scope []string) SafetyPolicy {
	narrowedScope := make([]string, len(scope))
	copy(narrowedScope, scope)

	// A sub-agent can never escalate ask to allow on its own
	mode := p.Mode
	if mode == "allow" && len(scope) > 0 {
		// Even if lead is allow, sub-agents check their scope strictly
	}

	return &StandardPolicy{
		Mode:      mode,
		Scope:     narrowedScope,
		Allowlist: p.Allowlist,
	}
}

var promptMu sync.Mutex

// Confirm asks the user for explicit permission before running an action.
func Confirm(ctx context.Context, target string, detail string) bool {
	promptMu.Lock()
	defer promptMu.Unlock()

	fmt.Printf("\n⚠️  [Goo Safety Gate] Permission requested:\n")
	fmt.Printf("  Target: %s\n", target)
	if len(detail) > 0 {
		dispDetail := detail
		if len(dispDetail) > 300 {
			dispDetail = dispDetail[:300] + "..."
		}
		fmt.Printf("  Detail: %s\n", dispDetail)
	}
	fmt.Print("  Confirm execution? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	type result struct {
		line string
		err  error
	}
	resCh := make(chan result, 1)
	go func() {
		line, err := reader.ReadString('\n')
		resCh <- result{line: line, err: err}
	}()

	select {
	case <-ctx.Done():
		fmt.Println("Cancelled by context.")
		return false
	case res := <-resCh:
		if res.err != nil {
			return false
		}
		ans := strings.TrimSpace(strings.ToLower(res.line))
		return ans == "y" || ans == "yes"
	}
}
