package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "plr",
	Short: "Pulumi Local Runner - run Pulumi apps from remote repositories",
	Long:  "plr clones remote git repositories containing Pulumi programs and runs them locally.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
