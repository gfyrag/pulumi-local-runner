package cmd

import (
	"github.com/gfyrag/plr/internal/config"
	"github.com/gfyrag/plr/internal/engine"
	"github.com/spf13/cobra"
)

func init() {
	var refresh bool
	upCmd := &cobra.Command{
		Use:     "up [app[/stack]...]",
		Aliases: []string{"deploy"},
		Short:   "Deploy stacks",
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			targets, err := engine.ResolveTargets(cfg, args)
			if err != nil {
				return err
			}
			if refresh {
				if err := engine.Run(cmd.Context(), cfg, targets, engine.OpRefresh, runOptions()); err != nil {
					return err
				}
			}
			return engine.Run(cmd.Context(), cfg, targets, engine.OpUp, runOptions())
		},
	}
	upCmd.Flags().BoolVarP(&refresh, "refresh", "r", false, "Run a refresh before deploying")
	rootCmd.AddCommand(upCmd)
}
