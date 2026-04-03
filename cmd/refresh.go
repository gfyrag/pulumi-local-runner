package cmd

import (
	"github.com/gfyrag/plr/internal/engine"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:               "refresh [app[/stack]...]",
		Aliases:           []string{"ref"},
		Short:             "Refresh stack state",
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: completeTargets,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := getStore()
			if err != nil {
				return err
			}
			cfg, err := s.LoadConfig()
			if err != nil {
				return err
			}
			targets, err := engine.ResolveTargets(cfg, args)
			if err != nil {
				return err
			}
			return engine.Run(cmd.Context(), s, cfg, targets, engine.OpRefresh, runOptions())
		},
	})
}
