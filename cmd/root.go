package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/gfyrag/plr/internal/engine"
	"github.com/spf13/cobra"
)

var (
	verbose       bool
	configValues  []string
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

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
