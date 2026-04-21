package cmd

import (
	"github.com/gfyrag/plr/internal/engine"
	"github.com/spf13/cobra"
)

func init() {
	var refresh bool
	previewCmd := &cobra.Command{
		Use:               "preview [app[/stack]...]",
		Aliases:           []string{"pre", "diff"},
		Short:             "Preview stack changes",
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
			if refresh {
				if err := engine.Run(cmd.Context(), s, cfg, targets, engine.OpRefresh, runOptions()); err != nil {
					return err
				}
			}
			return engine.Run(cmd.Context(), s, cfg, targets, engine.OpPreview, runOptions())
		},
	}
	previewCmd.Flags().BoolVarP(&refresh, "refresh", "r", false, "Refresh state from provider before preview")
	rootCmd.AddCommand(previewCmd)
}
