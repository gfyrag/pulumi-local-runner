package cmd

import (
	"github.com/gfyrag/plr/internal/config"
	"github.com/gfyrag/plr/internal/engine"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "refresh [app[/stack]...]",
		Short: "Refresh stack state",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			targets, err := engine.ResolveTargets(cfg, args)
			if err != nil {
				return err
			}
			return engine.Run(cmd.Context(), cfg, targets, engine.OpRefresh, runOptions())
		},
	})
}
