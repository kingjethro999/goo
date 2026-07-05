package orchestrator

import (
	"context"
	"sync"

	"github.com/kingjethro999/goo/internal/agent"
)

type Orchestrator struct {
	lead       *agent.Core
	maxAgents  int
	policy     agent.SafetyPolicy
	toolsFor   ToolProvider
	testRunner func(ctx context.Context, prompt string, c *agent.Core) ([]agent.FileDiff, []agent.Turn, error)
}

func New(lead *agent.Core, maxAgents int, policy agent.SafetyPolicy, tp ToolProvider) *Orchestrator {
	if maxAgents <= 0 {
		maxAgents = 4
	}
	return &Orchestrator{lead: lead, maxAgents: maxAgents, policy: policy, toolsFor: tp}
}

func (o *Orchestrator) SetTestRunner(tr func(ctx context.Context, prompt string, c *agent.Core) ([]agent.FileDiff, []agent.Turn, error)) {
	o.testRunner = tr
}

func (o *Orchestrator) RunParallel(ctx context.Context, tasks []Task) []TaskResult {
	results := make([]TaskResult, len(tasks))
	sem := make(chan struct{}, o.maxAgents)
	var wg sync.WaitGroup

	for i, t := range tasks {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, t Task) {
			defer wg.Done()
			defer func() { <-sem }()

			subCtx, cancel := context.WithCancel(ctx)
			GlobalStatus.RegisterCancel(t.ID, cancel)

			var subTools []agent.FileDiff // placeholder to avoid syntax errors
			_ = subTools
			
			sub := agent.NewCore(agent.CoreConfig{
				Tools:      o.toolsFor.ToolsForScope(t.Scope),
				Policy:     o.policy.Narrow(t.Scope),
				TestRunner: o.testRunner,
			})
			results[i] = runTask(subCtx, sub, t)
			status := "done"
			if results[i].Err != nil {
				status = "failed"
				results[i].ErrMsg = results[i].Err.Error()
			}
			GlobalStatus.UpdateTask(GlobalStatus.activeRun, t.ID, status, results[i].Err)
		}(i, t)
	}
	wg.Wait()
	return results
}

func runTask(ctx context.Context, sub *agent.Core, t Task) TaskResult {
	diffs, log, err := sub.RunToCompletion(ctx, t.Description)
	return TaskResult{TaskID: t.ID, Diffs: diffs, Log: log, Err: err}
}
