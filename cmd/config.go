package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
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

	// config schema
	configCmd.AddCommand(&cobra.Command{
		Use:               "schema <app/stack>",
		Short:             "Show available config keys for a stack's Pulumi project",
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

			if err := git.Sync(s, app, stack); err != nil {
				return fmt.Errorf("syncing repo: %w", err)
			}

			workDir, err := git.WorkDir(app)
			if err != nil {
				return err
			}

			schema, err := pulumibridge.LoadConfigSchema(workDir)
			if err != nil {
				return err
			}
			if schema == nil {
				fmt.Println("No config schema defined in Pulumi.yaml")
				return nil
			}

			printEntries(schema.Entries, 1)
			return nil
		},
	})

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

			ps, cleanup, err := getStackWithConfig(cmd, s, app, stack)
			if err != nil {
				return err
			}
			defer cleanup()

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

			ps, cleanup, err := getStackWithConfig(cmd, s, app, stack)
			if err != nil {
				return err
			}
			defer cleanup()

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

			ps, cleanup, err := getStackWithConfig(cmd, s, app, stack)
			if err != nil {
				return err
			}
			defer cleanup()

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

			ps, cleanup, err := getStackWithConfig(cmd, s, app, stack)
			if err != nil {
				return err
			}
			defer cleanup()

			if err := pulumibridge.RemoveConfig(cmd.Context(), ps, args[1]); err != nil {
				return err
			}

			return git.SaveStackConfig(s, app, stack)
		},
	})
}

// getStackWithConfig restores the config (with base merging) into a temp workdir, then returns the Pulumi stack.
// The caller must call the returned cleanup function when done.
func getStackWithConfig(cmd *cobra.Command, s store.Store, app *config.App, stack *config.Stack) (pulumibridge.Stack, func(), error) {
	git.EnsurePassphrase()

	workDir, cleanup, err := git.PrepareWorkDir(s, app, stack)
	if err != nil {
		return pulumibridge.Stack{}, nil, fmt.Errorf("preparing workdir: %w", err)
	}

	ps, err := pulumibridge.GetStack(cmd.Context(), stack, workDir)
	if err != nil {
		cleanup()
		return pulumibridge.Stack{}, nil, err
	}

	return ps, cleanup, nil
}

var (
	schemaKey      = color.New(color.Bold, color.FgCyan)
	schemaType     = color.New(color.FgYellow)
	schemaDefault  = color.New(color.FgGreen)
	schemaRequired = color.New(color.Bold, color.FgRed)
	schemaSecret   = color.New(color.FgMagenta)
	schemaDesc     = color.New(color.Faint)
)

func printEntries(entries []pulumibridge.ConfigSchemaEntry, depth int) {
	// Compute the max key+type width for alignment
	maxLeft := 0
	for _, e := range entries {
		w := depth*2 + len(e.Key) + 2 + len(e.Type)
		if w > maxLeft {
			maxLeft = w
		}
	}
	if maxLeft < 30 {
		maxLeft = 30
	}

	for _, e := range entries {
		indent := strings.Repeat("  ", depth)

		// Build the left part: key + type
		left := fmt.Sprintf("%s%s  %s", indent, e.Key, e.Type)

		// Build the right part: tags + description
		var tags []string
		if e.HasDefault() {
			tags = append(tags, fmt.Sprintf("default: %v", e.Default))
		}
		if e.Secret {
			tags = append(tags, "secret")
		}
		if e.Required {
			tags = append(tags, "required")
		}

		// Print key and type
		schemaKey.Print(indent + e.Key)
		fmt.Print("  ")
		schemaType.Print(e.Type)

		// Pad to align
		padding := maxLeft - len(left)
		if padding < 2 {
			padding = 2
		}
		fmt.Print(strings.Repeat(" ", padding))

		// Print tags
		for i, tag := range tags {
			if i > 0 {
				fmt.Print(" ")
			}
			switch tag {
			case "required":
				schemaRequired.Printf("[%s]", tag)
			case "secret":
				schemaSecret.Printf("[%s]", tag)
			default:
				schemaDefault.Printf("[%s]", tag)
			}
		}

		// Print description
		if e.Description != "" {
			if len(tags) > 0 {
				fmt.Print("  ")
			}
			schemaDesc.Print(e.Description)
		}
		fmt.Println()

		if len(e.Properties) > 0 {
			printEntries(e.Properties, depth+1)
		}
	}
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
