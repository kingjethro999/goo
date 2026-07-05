package orchestrator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kingjethro999/goo/internal/agent"
)

type MergeReport struct {
	Conflicts  []string         `json:"conflicts"`
	CleanDiffs []agent.FileDiff `json:"clean_diffs"`
	Summary    string           `json:"summary"`
}

func Merge(results []TaskResult) *MergeReport {
	report := &MergeReport{}
	touched := make(map[string]string) // path -> TaskID
	diffMap := make(map[string]agent.FileDiff)
	conflictSet := make(map[string]bool)

	var successCount, failCount int

	for _, res := range results {
		if res.Err != nil {
			failCount++
		} else {
			successCount++
		}

		for _, diff := range res.Diffs {
			if prevTask, found := touched[diff.Path]; found && prevTask != res.TaskID {
				conflictSet[diff.Path] = true
			} else {
				touched[diff.Path] = res.TaskID
				diffMap[diff.Path] = diff
			}
		}
	}

	for path := range conflictSet {
		report.Conflicts = append(report.Conflicts, path)
		delete(diffMap, path)
	}
	sort.Strings(report.Conflicts)

	for _, diff := range diffMap {
		report.CleanDiffs = append(report.CleanDiffs, diff)
	}
	sort.Slice(report.CleanDiffs, func(i, j int) bool {
		return report.CleanDiffs[i].Path < report.CleanDiffs[j].Path
	})

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Orchestration finished: %d tasks succeeded, %d tasks failed.\n", successCount, failCount))
	if len(report.Conflicts) > 0 {
		sb.WriteString(fmt.Sprintf("CONFLICTS DETECTED in %d file(s): %s\n", len(report.Conflicts), strings.Join(report.Conflicts, ", ")))
	}
	sb.WriteString(fmt.Sprintf("Clean diffs ready to apply: %d file(s).\n", len(report.CleanDiffs)))
	report.Summary = sb.String()

	return report
}
