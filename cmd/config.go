package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gfyrag/plr/internal/config"
	"github.com/gfyrag/plr/internal/git"
	pulumibridge "github.com/gfyrag/plr/internal/pulumi"
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
		Use:   "set <app/stack> <key> <value>",
		Short: "Set a config value",
		Args:  cobra.ExactArgs(3),
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

			return pulumibridge.SetConfig(cmd.Context(), s, args[1], args[2], setSecret)
		},
	}
	setCmd.Flags().BoolVar(&setSecret, "secret", false, "Mark the value as secret")
	configCmd.AddCommand(setCmd)

	// config get
	configCmd.AddCommand(&cobra.Command{
		Use:   "get <app/stack> <key>",
		Short: "Get a config value",
		Args:  cobra.ExactArgs(2),
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

			val, err := pulumibridge.GetConfig(cmd.Context(), s, args[1])
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
		Use:   "list <app/stack>",
		Short: "List all config values",
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

			all, err := pulumibridge.GetAllConfig(cmd.Context(), s)
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

	// config import
	configCmd.AddCommand(&cobra.Command{
		Use:   "import <app/stack> <path-to-Pulumi.stack.yaml>",
		Short: "Import a Pulumi stack config file",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, stack, err := resolveAppStack(args[0])
			if err != nil {
				return err
			}

			srcPath := args[1]
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return fmt.Errorf("reading %s: %w", srcPath, err)
			}

			workDir, err := git.WorkDir(app)
			if err != nil {
				return err
			}

			destPath := filepath.Join(workDir, fmt.Sprintf("Pulumi.%s.yaml", stack.Name))
			if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
				return err
			}

			if err := os.WriteFile(destPath, data, 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", destPath, err)
			}

			fmt.Printf("Imported config to %s\n", destPath)
			return nil
		},
	})

	// config edit
	configCmd.AddCommand(&cobra.Command{
		Use:   "edit <app/stack>",
		Short: "Open the stack config file in your editor",
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

			configFile := filepath.Join(workDir, fmt.Sprintf("Pulumi.%s.yaml", stack.Name))

			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}

			c := exec.Command(editor, configFile)
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			return c.Run()
		},
	})

	// config rm
	configCmd.AddCommand(&cobra.Command{
		Use:   "rm <app/stack> <key>",
		Short: "Remove a config value",
		Args:  cobra.ExactArgs(2),
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

			return pulumibridge.RemoveConfig(cmd.Context(), s, args[1])
		},
	})
}

func resolveAppStack(target string) (*config.App, *config.Stack, error) {
	parts := strings.SplitN(target, "/", 2)
	if len(parts) != 2 {
		return nil, nil, fmt.Errorf("expected app/stack format, got %q", target)
	}

	cfg, err := config.Load()
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
