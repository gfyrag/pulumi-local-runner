package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/gfyrag/plr/internal/config"
	"github.com/gfyrag/plr/internal/git"
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
			s, err := getStore()
			if err != nil {
				return err
			}

			cfg, err := s.LoadConfig()
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

			if err := validateAppPaths(&app); err != nil {
				return err
			}

			cfg.Apps = append(cfg.Apps, app)
			if err := s.SaveConfig(cfg); err != nil {
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
		Use:               "remove <name>",
		Aliases:           []string{"rm"},
		Short:             "Remove an app",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeApps,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := getStore()
			if err != nil {
				return err
			}

			cfg, err := s.LoadConfig()
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
			if err := s.SaveConfig(cfg); err != nil {
				return err
			}
			fmt.Printf("Removed app %q\n", name)
			return nil
		},
	})

	// app rename
	appCmd.AddCommand(&cobra.Command{
		Use:               "rename <old-name> <new-name>",
		Aliases:           []string{"mv"},
		Short:             "Rename an app",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeApps,
		RunE: func(cmd *cobra.Command, args []string) error {
			oldName, newName := args[0], args[1]

			s, err := getStore()
			if err != nil {
				return err
			}

			cfg, err := s.LoadConfig()
			if err != nil {
				return err
			}

			if _, err := cfg.FindApp(newName); err == nil {
				return fmt.Errorf("app %q already exists", newName)
			}

			app, err := cfg.FindApp(oldName)
			if err != nil {
				return err
			}

			// Update dependsOn references across all stacks
			for i := range cfg.Apps {
				for j := range cfg.Apps[i].Stacks {
					for k, dep := range cfg.Apps[i].Stacks[j].DependsOn {
						parts := strings.SplitN(dep, "/", 2)
						if len(parts) == 2 && parts[0] == oldName {
							cfg.Apps[i].Stacks[j].DependsOn[k] = newName + "/" + parts[1]
						}
					}
				}
			}

			// Rename the app directory before SaveConfig so configs are preserved
			if oldPath, err := s.StackFilePath(oldName, "app"); err == nil {
				oldDir := filepath.Dir(oldPath)
				newDir := filepath.Join(filepath.Dir(oldDir), newName)
				os.Rename(oldDir, newDir)
			}

			app.Name = newName

			if err := s.SaveConfig(cfg); err != nil {
				return err
			}

			fmt.Printf("Renamed app %q → %q\n", oldName, newName)
			return nil
		},
	})

	// app set
	var setRepo, setPath string
	setCmd := &cobra.Command{
		Use:               "set <name>",
		Short:             "Update an app's repo or path",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeApps,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := getStore()
			if err != nil {
				return err
			}

			cfg, err := s.LoadConfig()
			if err != nil {
				return err
			}

			app, err := cfg.FindApp(args[0])
			if err != nil {
				return err
			}

			if cmd.Flags().Changed("repo") {
				app.Repo = setRepo
			}
			if cmd.Flags().Changed("path") {
				app.Path = setPath
			}

			if err := validateAppPaths(app); err != nil {
				return err
			}

			if err := s.SaveConfig(cfg); err != nil {
				return err
			}
			fmt.Printf("Updated app %q\n", args[0])
			return nil
		},
	}
	setCmd.Flags().StringVar(&setRepo, "repo", "", "Git repository URL")
	setCmd.Flags().StringVar(&setPath, "path", "", "Subdirectory within the repo")
	appCmd.AddCommand(setCmd)

	// app list
	appCmd.AddCommand(&cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all apps",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := getStore()
			if err != nil {
				return err
			}

			cfg, err := s.LoadConfig()
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
				for i, st := range app.Stacks {
					if i > 0 {
						fmt.Print(", ")
					}
					fmt.Print(st.Name)
				}
				fmt.Println()
			}
			return nil
		},
	})
}

// validateAppPaths checks that repo and path exist for local repos.
func validateAppPaths(app *config.App) error {
	if !git.IsLocalRepo(app.Repo) {
		return nil
	}

	repoDir, err := git.RepoDir(app)
	if err != nil {
		return err
	}

	if info, err := os.Stat(repoDir); err != nil || !info.IsDir() {
		return fmt.Errorf("repo directory does not exist: %s", repoDir)
	}

	workDir := filepath.Join(repoDir, app.Path)
	if info, err := os.Stat(workDir); err != nil || !info.IsDir() {
		return fmt.Errorf("path %q does not exist in repo %s", app.Path, repoDir)
	}

	pulumiYaml := filepath.Join(workDir, "Pulumi.yaml")
	if _, err := os.Stat(pulumiYaml); err != nil {
		return fmt.Errorf("no Pulumi.yaml found at %s", workDir)
	}

	return nil
}
