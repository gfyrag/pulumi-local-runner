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

			s, err := getStackWithConfig(cmd, app, stack)
			if err != nil {
				return err
			}

			if err := pulumibridge.SetConfig(cmd.Context(), s, args[1], args[2], setSecret); err != nil {
				return err
			}

			return git.SaveStackConfig(app, stack)
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

			s, err := getStackWithConfig(cmd, app, stack)
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
		Use:     "list <app/stack>",
		Aliases: []string{"ls"},
		Short:   "List all config values",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, stack, err := resolveAppStack(args[0])
			if err != nil {
				return err
			}

			s, err := getStackWithConfig(cmd, app, stack)
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

	// config import — writes directly to config store
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

			storePath, err := config.StackConfigPath(app.Name, stack.Name)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(storePath, data, 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", storePath, err)
			}

			fmt.Printf("Imported config to %s\n", storePath)
			return nil
		},
	})

	// config edit — edits the config store file directly
	configCmd.AddCommand(&cobra.Command{
		Use:   "edit <app/stack>",
		Short: "Open the stack config file in your editor",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, stack, err := resolveAppStack(args[0])
			if err != nil {
				return err
			}

			storePath, err := config.StackConfigPath(app.Name, stack.Name)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
				return err
			}

			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}

			c := exec.Command(editor, storePath)
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

			s, err := getStackWithConfig(cmd, app, stack)
			if err != nil {
				return err
			}

			if err := pulumibridge.RemoveConfig(cmd.Context(), s, args[1]); err != nil {
				return err
			}

			return git.SaveStackConfig(app, stack)
		},
	})
}

// getStackWithConfig restores the config into workdir, then returns the Pulumi stack.
func getStackWithConfig(cmd *cobra.Command, app *config.App, stack *config.Stack) (pulumibridge.Stack, error) {
	workDir, err := git.WorkDir(app)
	if err != nil {
		return pulumibridge.Stack{}, err
	}

	// Restore config from store to workdir before accessing via Automation API
	storePath, err := config.StackConfigPath(app.Name, stack.Name)
	if err != nil {
		return pulumibridge.Stack{}, err
	}
	if data, readErr := os.ReadFile(storePath); readErr == nil {
		dest := filepath.Join(workDir, fmt.Sprintf("Pulumi.%s.yaml", stack.Name))
		if writeErr := os.WriteFile(dest, data, 0o644); writeErr != nil {
			return pulumibridge.Stack{}, fmt.Errorf("restoring config to workdir: %w", writeErr)
		}
	}

	return pulumibridge.GetStack(cmd.Context(), stack, workDir)
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
