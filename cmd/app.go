package cmd

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/gfyrag/plr/internal/config"
	"github.com/spf13/cobra"
)

var (
	appBold = color.New(color.Bold, color.FgCyan)
	appDim  = color.New(color.Faint)
)

var appCmd = &cobra.Command{
	Use:   "app",
	Short: "Manage apps",
}

func init() {
	rootCmd.AddCommand(appCmd)

	// app add
	var repo, path string
	addCmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add an app",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			name := args[0]
			if _, err := cfg.FindApp(name); err == nil {
				return fmt.Errorf("app %q already exists", name)
			}

			app := config.App{
				Name: name,
				Repo: repo,
				Path: path,
			}

			cfg.Apps = append(cfg.Apps, app)
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Printf("Added app %q\n", name)
			return nil
		},
	}
	addCmd.Flags().StringVar(&repo, "repo", "", "Git repository URL (required)")
	addCmd.Flags().StringVar(&path, "path", ".", "Subdirectory within the repo")
	addCmd.MarkFlagRequired("repo")
	appCmd.AddCommand(addCmd)

	// app remove
	appCmd.AddCommand(&cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm"},
		Short:   "Remove an app",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			name := args[0]
			idx := -1
			for i, a := range cfg.Apps {
				if a.Name == name {
					idx = i
					break
				}
			}
			if idx == -1 {
				return fmt.Errorf("app %q not found", name)
			}

			cfg.Apps = append(cfg.Apps[:idx], cfg.Apps[idx+1:]...)
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Printf("Removed app %q\n", name)
			return nil
		},
	})

	// app list
	appCmd.AddCommand(&cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all apps",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			if len(cfg.Apps) == 0 {
				fmt.Println("No apps configured.")
				return nil
			}

			for _, app := range cfg.Apps {
				appBold.Println(app.Name)
				appDim.Printf("  repo:   %s\n", app.Repo)
				appDim.Printf("  path:   %s\n", app.Path)
				fmt.Print("  stacks: ")
				if len(app.Stacks) == 0 {
					appDim.Print("(none)")
				}
				for i, s := range app.Stacks {
					if i > 0 {
						fmt.Print(", ")
					}
					fmt.Print(s.Name)
				}
				fmt.Println()
			}
			return nil
		},
	})
}
