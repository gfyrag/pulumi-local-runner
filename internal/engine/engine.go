package engine

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gfyrag/plr/internal/config"
	"github.com/gfyrag/plr/internal/git"
	pulumibridge "github.com/gfyrag/plr/internal/pulumi"
	"github.com/gfyrag/plr/internal/store"
	"github.com/gfyrag/plr/internal/ui"
	"gopkg.in/yaml.v3"
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

// RunOptions holds runtime options for a Pulumi operation.
type RunOptions struct {
	Verbose       bool
	ConfigOverrides map[string]string // key=value pairs applied before the run, removed after
}

// Run executes the given operation on the resolved targets, respecting dependency order.
func Run(ctx context.Context, s store.Store, cfg *config.Config, targets []Target, op Operation, opts RunOptions) error {
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
		if err := runOne(ctx, s, t, op, opts); err != nil {
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

func runOne(ctx context.Context, s store.Store, t Target, op Operation, opts RunOptions) error {
	ui.Header(t.Key(), op.String())

	// Ensure PULUMI_CONFIG_PASSPHRASE is set so Pulumi can decrypt secrets
	git.EnsurePassphrase()

	if err := git.Sync(s, t.App, t.Stack); err != nil {
		return fmt.Errorf("git sync: %w", err)
	}

	if op == OpSync {
		return nil
	}

	realWorkDir, err := git.WorkDir(t.App)
	if err != nil {
		return err
	}

	// Persist -c overrides to the store before building the merged config
	if len(opts.ConfigOverrides) > 0 {
		if err := persistConfigOverrides(s, t.App, t.Stack, opts.ConfigOverrides, realWorkDir); err != nil {
			return fmt.Errorf("persisting config overrides: %w", err)
		}
	}

	// Create a temp workdir with symlinks + merged config (avoids polluting the real repo)
	workDir, cleanup, err := git.PrepareWorkDir(s, t.App, t.Stack)
	if err != nil {
		return fmt.Errorf("preparing workdir: %w", err)
	}
	defer cleanup()

	stack, err := pulumibridge.GetStack(ctx, t.Stack, workDir)
	if err != nil {
		return fmt.Errorf("getting stack: %w", err)
	}

	// Validate config against Pulumi.yaml schema
	if op == OpUp || op == OpPreview {
		if schema, err := pulumibridge.LoadConfigSchema(workDir); err == nil && schema != nil {
			allConfig, configErr := pulumibridge.GetAllConfig(ctx, stack)
			if configErr == nil {
				configMap := make(map[string]any)
				for k := range allConfig {
					configMap[k] = true
				}
				result := pulumibridge.ValidateConfig(schema, configMap)
				for _, key := range result.Unknown {
					ui.Warn("Unknown config key: %s", key)
				}
				for _, key := range result.Missing {
					ui.Warn("Missing required config key: %s", key)
				}
				if len(result.Missing) > 0 && op == OpUp {
					return fmt.Errorf("missing required config keys: %v", result.Missing)
				}
			}
		}
	}

	// Cancel the Pulumi operation if the context is interrupted (ctrl-c),
	// so the stack lock is released and no pending operation is left behind.
	done := make(chan struct{})
	go func() {
		select {
		case <-done:
		case <-ctx.Done():
			ui.Warn("Interrupted, cancelling Pulumi operation...")
			// Use a background context since the original is already cancelled.
			if cancelErr := stack.Cancel(context.Background()); cancelErr != nil {
				ui.Warn("Cancel failed: %s", cancelErr)
			}
		}
	}()

	var pulumiErr error
	switch op {
	case OpUp:
		pulumiErr = pulumibridge.Up(ctx, stack, opts.Verbose)
	case OpPreview:
		pulumiErr = pulumibridge.Preview(ctx, stack, opts.Verbose)
	case OpDestroy:
		pulumiErr = pulumibridge.Destroy(ctx, stack, opts.Verbose)
	case OpRefresh:
		pulumiErr = pulumibridge.Refresh(ctx, stack, opts.Verbose)
	default:
		close(done)
		return fmt.Errorf("unknown operation: %s", op)
	}
	close(done)

	return pulumiErr
}

// persistConfigOverrides writes -c overrides into the stack config in the store.
// It uses the schema to convert values to the correct type.
func persistConfigOverrides(s store.Store, app *config.App, stack *config.Stack, overrides map[string]string, workDir string) error {
	data, err := s.ReadStackConfig(app.Name, stack.Name)
	if err != nil {
		return err
	}

	var m map[string]any
	if data != nil {
		if err := yaml.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("parsing stack config: %w", err)
		}
	}
	if m == nil {
		m = make(map[string]any)
	}

	cfg, _ := m["config"].(map[string]any)
	if cfg == nil {
		cfg = make(map[string]any)
	}

	// Load schema for type coercion
	schema, _ := pulumibridge.LoadConfigSchema(workDir)

	for k, v := range overrides {
		typed := coerceValue(v, resolveType(schema, k))
		setNestedValue(cfg, k, typed)
	}

	m["config"] = cfg
	out, err := yaml.Marshal(m)
	if err != nil {
		return err
	}

	return s.WriteStackConfig(app.Name, stack.Name, out)
}

// resolveType finds the schema type for a dot-separated key path.
func resolveType(schema *pulumibridge.ConfigSchema, key string) string {
	if schema == nil {
		return "string"
	}
	parts := strings.Split(key, ".")
	entries := schema.Entries
	for i, part := range parts {
		for _, e := range entries {
			if e.Key == part {
				if i == len(parts)-1 {
					return e.Type
				}
				entries = e.Properties
				break
			}
		}
	}
	return "string"
}

// coerceValue converts a string value to the appropriate Go type based on the schema type.
func coerceValue(value, typ string) any {
	switch typ {
	case "boolean":
		return value == "true" || value == "1"
	case "integer":
		n, err := strconv.Atoi(value)
		if err == nil {
			return n
		}
		return value
	default:
		return value
	}
}

// setNestedValue sets a value at a dot-separated path in a map.
func setNestedValue(m map[string]any, key string, value any) {
	parts := strings.Split(key, ".")
	if len(parts) == 1 {
		m[key] = value
		return
	}

	// Navigate/create nested maps
	current := m
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[part] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
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
