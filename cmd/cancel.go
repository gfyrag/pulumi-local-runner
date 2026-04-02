package cmd

import (
	"fmt"

	"github.com/gfyrag/plr/internal/git"
	pulumibridge "github.com/gfyrag/plr/internal/pulumi"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "cancel <app/stack>",
		Short: "Cancel a running Pulumi operation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, stack, err := resolveAppStack(args[0])
			if err != nil {
				return err
			}

			workDir, err := git.WorkDir(app)
			if err != nil {
				return err
			}

			s, err := pulumibridge.GetStack(cmd.Context(), stack, workDir)
			if err != nil {
				return err
			}

			if err := s.Cancel(cmd.Context()); err != nil {
				return fmt.Errorf("cancel failed: %w", err)
			}

			fmt.Printf("Cancelled running operation on %s/%s\n", app.Name, stack.Name)
			return nil
		},
	})
}
