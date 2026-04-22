package cmd

import (
	"github.com/gfyrag/plr/internal/engine"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:               "destroy [app[/stack]...]",
		Aliases:           []string{"down"},
		Short:             "Destroy stacks",
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
			targets, err := engine.ResolveTargets(cfg, args, resolveEnv())
			if err != nil {
				return err
			}
			return engine.Run(cmd.Context(), s, cfg, targets, engine.OpDestroy, runOptions())
		},
	})
}
