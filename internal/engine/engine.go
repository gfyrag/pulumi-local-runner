package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/gfyrag/plr/internal/config"
	"github.com/gfyrag/plr/internal/git"
	pulumibridge "github.com/gfyrag/plr/internal/pulumi"
	"github.com/gfyrag/plr/internal/ui"
)

// Target represents an app/stack pair to operate on.
type Target struct {
	App   *config.App
	Stack *config.Stack
}

func (t Target) Key() string {
	return t.App.Name + "/" + t.Stack.Name
}

type Operation int

const (
	OpUp Operation = iota
	OpPreview
	OpDestroy
	OpRefresh
	OpSync
)

func (o Operation) String() string {
	switch o {
	case OpUp:
		return "up"
	case OpPreview:
		return "preview"
	case OpDestroy:
		return "destroy"
	case OpRefresh:
		return "refresh"
	case OpSync:
		return "sync"
	default:
		return "unknown"
	}
}

// ResolveTargets parses arguments like "networking", "networking/dev" into targets.
// If no args are given, all app/stack pairs are returned.
func ResolveTargets(cfg *config.Config, args []string) ([]Target, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	if len(args) == 0 {
		var targets []Target
		for i := range cfg.Apps {
			for j := range cfg.Apps[i].Stacks {
				targets = append(targets, Target{
					App:   &cfg.Apps[i],
					Stack: &cfg.Apps[i].Stacks[j],
				})
			}
		}
		return targets, nil
	}

	var targets []Target
	for _, arg := range args {
		parts := strings.SplitN(arg, "/", 2)
		app, err := cfg.FindApp(parts[0])
		if err != nil {
			return nil, err
		}

		if len(parts) == 2 {
			stack, err := app.FindStack(parts[1])
			if err != nil {
				return nil, err
			}
			targets = append(targets, Target{App: app, Stack: stack})
		} else {
			for j := range app.Stacks {
				targets = append(targets, Target{App: app, Stack: &app.Stacks[j]})
			}
		}
	}
	return targets, nil
}

// Run executes the given operation on the resolved targets, respecting dependency order.
func Run(ctx context.Context, cfg *config.Config, targets []Target, op Operation) error {
	if len(targets) == 0 {
		ui.Warn("No targets to run.")
		return nil
	}

	ordered, err := topoSort(targets)
	if err != nil {
		return err
	}

	succeeded := 0
	failed := 0
	var errs []string
	for _, t := range ordered {
		if err := runOne(ctx, t, op); err != nil {
			failed++
			errs = append(errs, fmt.Sprintf("%s: %s", t.Key(), err))
			ui.ResultFail(t.Key(), err.Error())
		} else {
			succeeded++
			ui.ResultOK(t.Key())
		}
	}

	ui.Summary(len(ordered), succeeded, failed)

	if len(errs) > 0 {
		return fmt.Errorf("failed targets:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

func runOne(ctx context.Context, t Target, op Operation) error {
	ui.Header(t.Key(), op.String())

	if err := git.Sync(t.App, t.Stack); err != nil {
		return fmt.Errorf("git sync: %w", err)
	}

	if op == OpSync {
		return nil
	}

	workDir, err := git.WorkDir(t.App)
	if err != nil {
		return err
	}

	stack, err := pulumibridge.GetStack(ctx, t.Stack, workDir)
	if err != nil {
		return fmt.Errorf("getting stack: %w", err)
	}

	switch op {
	case OpUp:
		return pulumibridge.Up(ctx, stack)
	case OpPreview:
		return pulumibridge.Preview(ctx, stack)
	case OpDestroy:
		return pulumibridge.Destroy(ctx, stack)
	case OpRefresh:
		return pulumibridge.Refresh(ctx, stack)
	default:
		return fmt.Errorf("unknown operation: %s", op)
	}
}

// topoSort orders targets respecting dependsOn. Simple Kahn's algorithm.
func topoSort(targets []Target) ([]Target, error) {
	index := make(map[string]int)
	for i, t := range targets {
		index[t.Key()] = i
	}

	inDegree := make([]int, len(targets))
	deps := make([][]int, len(targets))
	for i, t := range targets {
		for _, dep := range t.Stack.DependsOn {
			j, ok := index[dep]
			if !ok {
				continue
			}
			deps[j] = append(deps[j], i)
			inDegree[i]++
		}
	}

	var queue []int
	for i, d := range inDegree {
		if d == 0 {
			queue = append(queue, i)
		}
	}

	var ordered []Target
	for len(queue) > 0 {
		i := queue[0]
		queue = queue[1:]
		ordered = append(ordered, targets[i])
		for _, j := range deps[i] {
			inDegree[j]--
			if inDegree[j] == 0 {
				queue = append(queue, j)
			}
		}
	}

	if len(ordered) != len(targets) {
		return nil, fmt.Errorf("circular dependency detected among targets")
	}
	return ordered, nil
}
