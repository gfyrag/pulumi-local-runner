package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gfyrag/plr/internal/config"
	"github.com/gfyrag/plr/internal/git"
	pulumibridge "github.com/gfyrag/plr/internal/pulumi"
	"github.com/gfyrag/plr/internal/store"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage Pulumi stack configuration",
}

func init() {
	rootCmd.AddCommand(configCmd)

	// config set
	setSecret := false
	setCmd := &cobra.Command{
		Use:               "set <app/stack> <key> <value>",
		Short:             "Set a config value",
		Args:              cobra.ExactArgs(3),
		ValidArgsFunction: completeAppStacks,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := getStore()
			if err != nil {
				return err
			}

			app, stack, err := resolveAppStack(s, args[0])
			if err != nil {
				return err
			}

			ps, err := getStackWithConfig(cmd, s, app, stack)
			if err != nil {
				return err
			}

			if err := pulumibridge.SetConfig(cmd.Context(), ps, args[1], args[2], setSecret); err != nil {
				return err
			}

			return git.SaveStackConfig(s, app, stack)
		},
	}
	setCmd.Flags().BoolVar(&setSecret, "secret", false, "Mark the value as secret")
	configCmd.AddCommand(setCmd)

	// config get
	configCmd.AddCommand(&cobra.Command{
		Use:               "get <app/stack> <key>",
		Short:             "Get a config value",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeAppStacks,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := getStore()
			if err != nil {
				return err
			}

			app, stack, err := resolveAppStack(s, args[0])
			if err != nil {
				return err
			}

			ps, err := getStackWithConfig(cmd, s, app, stack)
			if err != nil {
				return err
			}

			val, err := pulumibridge.GetConfig(cmd.Context(), ps, args[1])
			if err != nil {
				return err
			}

			if val.Secret {
				fmt.Println("[secret]")
			} else {
				fmt.Println(val.Value)
			}
			return nil
		},
	})

	// config list
	configCmd.AddCommand(&cobra.Command{
		Use:               "list <app/stack>",
		Aliases:           []string{"ls"},
		Short:             "List all config values",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeAppStacks,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := getStore()
			if err != nil {
				return err
			}

			app, stack, err := resolveAppStack(s, args[0])
			if err != nil {
				return err
			}

			ps, err := getStackWithConfig(cmd, s, app, stack)
			if err != nil {
				return err
			}

			all, err := pulumibridge.GetAllConfig(cmd.Context(), ps)
			if err != nil {
				return err
			}

			for k, v := range all {
				if v.Secret {
					fmt.Printf("%s = [secret]\n", k)
				} else {
					fmt.Printf("%s = %s\n", k, v.Value)
				}
			}
			return nil
		},
	})

	// config import — reads a local file and writes to store
	configCmd.AddCommand(&cobra.Command{
		Use:               "import <app/stack> <path-to-Pulumi.stack.yaml>",
		Short:             "Import a Pulumi stack config file",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeAppStacks,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := getStore()
			if err != nil {
				return err
			}

			app, stack, err := resolveAppStack(s, args[0])
			if err != nil {
				return err
			}

			srcPath := args[1]
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return fmt.Errorf("reading %s: %w", srcPath, err)
			}

			if err := s.WriteStackConfig(app.Name, stack.Name, data); err != nil {
				return fmt.Errorf("writing stack config: %w", err)
			}

			fmt.Printf("Imported config for %s/%s\n", app.Name, stack.Name)
			return nil
		},
	})

	// config rm
	configCmd.AddCommand(&cobra.Command{
		Use:               "rm <app/stack> <key>",
		Short:             "Remove a config value",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeAppStacks,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := getStore()
			if err != nil {
				return err
			}

			app, stack, err := resolveAppStack(s, args[0])
			if err != nil {
				return err
			}

			ps, err := getStackWithConfig(cmd, s, app, stack)
			if err != nil {
				return err
			}

			if err := pulumibridge.RemoveConfig(cmd.Context(), ps, args[1]); err != nil {
				return err
			}

			return git.SaveStackConfig(s, app, stack)
		},
	})
}

// getStackWithConfig restores the config (with base merging) into workdir, then returns the Pulumi stack.
func getStackWithConfig(cmd *cobra.Command, s store.Store, app *config.App, stack *config.Stack) (pulumibridge.Stack, error) {
	git.EnsurePassphrase()
	workDir, err := git.WorkDir(app)
	if err != nil {
		return pulumibridge.Stack{}, err
	}

	data, err := git.BuildMergedConfig(s, app, stack)
	if err != nil {
		return pulumibridge.Stack{}, fmt.Errorf("building merged config: %w", err)
	}
	if data != nil {
		dest := filepath.Join(workDir, fmt.Sprintf("Pulumi.%s.yaml", stack.Name))
		if writeErr := os.WriteFile(dest, data, 0o644); writeErr != nil {
			return pulumibridge.Stack{}, fmt.Errorf("restoring config to workdir: %w", writeErr)
		}
	}

	return pulumibridge.GetStack(cmd.Context(), stack, workDir)
}

func resolveAppStack(s store.Store, target string) (*config.App, *config.Stack, error) {
	parts := strings.SplitN(target, "/", 2)
	if len(parts) != 2 {
		return nil, nil, fmt.Errorf("expected app/stack format, got %q", target)
	}

	cfg, err := s.LoadConfig()
	if err != nil {
		return nil, nil, err
	}

	app, err := cfg.FindApp(parts[0])
	if err != nil {
		return nil, nil, err
	}

	stack, err := app.FindStack(parts[1])
	if err != nil {
		return nil, nil, err
	}

	return app, stack, nil
}
