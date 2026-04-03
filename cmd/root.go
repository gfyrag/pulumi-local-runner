package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gfyrag/plr/internal/engine"
	"github.com/gfyrag/plr/internal/store"
	"github.com/spf13/cobra"
)

var (
	verbose      bool
	configValues []string
)

var rootCmd = &cobra.Command{
	Use:   "plr",
	Short: "Pulumi Local Runner - run Pulumi apps from remote repositories",
	Long:  "plr clones remote git repositories containing Pulumi programs and runs them locally.",
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Show full Pulumi output")
	rootCmd.PersistentFlags().StringArrayVarP(&configValues, "config", "c", nil, "Config overrides (key=value, repeatable)")
}

func runOptions() engine.RunOptions {
	overrides := make(map[string]string)
	for _, kv := range configValues {
		k, v, ok := strings.Cut(kv, "=")
		if ok {
			overrides[k] = v
		}
	}
	return engine.RunOptions{
		Verbose:         verbose,
		ConfigOverrides: overrides,
	}
}

// getStore creates the config store based on backend.yaml configuration.
func getStore() (store.Store, error) {
	return store.NewStoreFromConfig(context.Background())
}

// completeTargets provides shell completion for app/stack targets.
func completeTargets(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	s, err := getStore()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	cfg, err := s.LoadConfig()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	existing := make(map[string]bool)
	for _, a := range args {
		existing[a] = true
	}

	var completions []string
	for _, app := range cfg.Apps {
		// Suggest app name (selects all stacks)
		if !existing[app.Name] && strings.HasPrefix(app.Name, toComplete) {
			completions = append(completions, app.Name)
		}
		// Suggest app/stack pairs
		for _, st := range app.Stacks {
			target := app.Name + "/" + st.Name
			if !existing[target] && strings.HasPrefix(target, toComplete) {
				completions = append(completions, target)
			}
		}
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
