package pulumi

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/events"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optdestroy"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optpreview"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optrefresh"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"gopkg.in/yaml.v3"

	"github.com/gfyrag/plr/internal/config"
	"github.com/gfyrag/plr/internal/ui"
)

// compactError strips the verbose stdout/stderr dump from Pulumi autoError
// messages, since event streaming already displayed diagnostics to the user.
func compactError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if idx := strings.Index(msg, "\ncode: "); idx >= 0 {
		return fmt.Errorf("%s", msg[:idx])
	}
	return err
}

// Stack is an alias for auto.Stack to avoid leaking the auto package in callers.
type Stack = auto.Stack

type pulumiProject struct {
	Name string `yaml:"name"`
}

func detectProjectName(workDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(workDir, "Pulumi.yaml"))
	if err != nil {
		return "", fmt.Errorf("reading Pulumi.yaml in %q: %w", workDir, err)
	}
	var proj pulumiProject
	if err := yaml.Unmarshal(data, &proj); err != nil {
		return "", fmt.Errorf("parsing Pulumi.yaml: %w", err)
	}
	if proj.Name == "" {
		return "", fmt.Errorf("Pulumi.yaml in %q has no project name", workDir)
	}
	return proj.Name, nil
}

// GetStack returns a Pulumi stack, creating it if it doesn't exist.
// If the stack config has an org, the fully qualified stack name (org/project/stack) is used.
func GetStack(ctx context.Context, stack *config.Stack, workDir string) (auto.Stack, error) {
	stackName := stack.Name

	if stack.Org != "" {
		project := stack.Project
		if project == "" {
			var err error
			project, err = detectProjectName(workDir)
			if err != nil {
				return auto.Stack{}, err
			}
		}
		stackName = fmt.Sprintf("%s/%s/%s", stack.Org, project, stack.Name)
	}

	ui.Info("Selecting stack %s...", stackName)
	s, err := auto.UpsertStackLocalSource(ctx, stackName, workDir)
	if err != nil {
		return s, fmt.Errorf("upsert stack %q in %q: %w", stackName, workDir, err)
	}
	return s, nil
}

func Up(ctx context.Context, stack auto.Stack, verbose bool) error {
	ui.Info("Running pulumi up...")
	if verbose {
		_, err := stack.Up(ctx,
			optup.ProgressStreams(os.Stdout),
			optup.ErrorProgressStreams(os.Stderr),
		)
		return compactError(err)
	}
	ch := make(chan events.EngineEvent)
	wg := streamEvents(ch)
	_, err := stack.Up(ctx, optup.EventStreams(ch))
	wg.Wait()
	return compactError(err)
}

func Preview(ctx context.Context, stack auto.Stack, verbose bool) error {
	ui.Info("Running pulumi preview...")
	if verbose {
		_, err := stack.Preview(ctx,
			optpreview.ProgressStreams(os.Stdout),
			optpreview.ErrorProgressStreams(os.Stderr),
			optpreview.Diff(),
		)
		return compactError(err)
	}
	ch := make(chan events.EngineEvent)
	wg := streamEvents(ch)
	_, err := stack.Preview(ctx, optpreview.EventStreams(ch), optpreview.Diff())
	wg.Wait()
	return compactError(err)
}

func Destroy(ctx context.Context, stack auto.Stack, verbose bool) error {
	ui.Info("Running pulumi destroy...")
	if verbose {
		_, err := stack.Destroy(ctx,
			optdestroy.ProgressStreams(os.Stdout),
			optdestroy.ErrorProgressStreams(os.Stderr),
		)
		return compactError(err)
	}
	ch := make(chan events.EngineEvent)
	wg := streamEvents(ch)
	_, err := stack.Destroy(ctx, optdestroy.EventStreams(ch))
	wg.Wait()
	return compactError(err)
}

func Refresh(ctx context.Context, stack auto.Stack, verbose bool) error {
	ui.Info("Running pulumi refresh...")
	if verbose {
		_, err := stack.Refresh(ctx,
			optrefresh.ProgressStreams(os.Stdout),
			optrefresh.ErrorProgressStreams(os.Stderr),
		)
		return compactError(err)
	}
	ch := make(chan events.EngineEvent)
	wg := streamEvents(ch)
	_, err := stack.Refresh(ctx, optrefresh.EventStreams(ch))
	wg.Wait()
	return compactError(err)
}

func SetConfig(ctx context.Context, stack auto.Stack, key, value string, secret bool) error {
	cv := auto.ConfigValue{
		Value:  value,
		Secret: secret,
	}
	// If the key contains a dot, treat it as a path (e.g., spec.image.tag)
	if strings.Contains(key, ".") {
		return stack.SetConfigWithOptions(ctx, key, cv, &auto.ConfigOptions{Path: true})
	}
	return stack.SetConfig(ctx, key, cv)
}

func GetConfig(ctx context.Context, stack auto.Stack, key string) (auto.ConfigValue, error) {
	return stack.GetConfig(ctx, key)
}

func GetAllConfig(ctx context.Context, stack auto.Stack) (auto.ConfigMap, error) {
	return stack.GetAllConfig(ctx)
}

func RemoveConfig(ctx context.Context, stack auto.Stack, key string) error {
	return stack.RemoveConfig(ctx, key)
}
