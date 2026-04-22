package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/fatih/color"
	"github.com/gfyrag/plr/internal/config"
	"github.com/spf13/cobra"
)

var (
	stackName = color.New(color.Bold, color.FgCyan)
	stackRef  = color.New(color.FgGreen)
	stackMeta = color.New(color.Faint)
)

var stackCmd = &cobra.Command{
	Use:   "stack",
	Short: "Manage stacks",
}

func init() {
	rootCmd.AddCommand(stackCmd)

	// stack add
	var branch, ref, org, addEnv string
	var dependsOn, bases []string
	addCmd := &cobra.Command{
		Use:               "add <app> <name>",
		Short:             "Add a stack to an app",
		Args:              cobra.ExactArgs(2),
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

			name := args[1]
			if _, err := app.FindStack(name); err == nil {
				return fmt.Errorf("stack %q already exists in app %q", name, app.Name)
			}

			if branch != "" && ref != "" {
				return fmt.Errorf("--branch and --ref are mutually exclusive")
			}

			env := addEnv
			if env == "" {
				env = resolveEnv()
			}
			if env == "" {
				env = "default"
			}

			stack := config.Stack{
				Name:      name,
				Env:       env,
				Branch:    branch,
				Ref:       ref,
				DependsOn: dependsOn,
				Org:       org,
				Bases:     bases,
			}

			app.Stacks = append(app.Stacks, stack)
			if err := s.SaveConfig(cfg); err != nil {
				return err
			}
			fmt.Printf("Added stack %s/%s\n", app.Name, name)
			return nil
		},
	}
	addCmd.Flags().StringVar(&branch, "branch", "", "Git branch to track")
	addCmd.Flags().StringVar(&ref, "ref", "", "Git ref (tag/commit) to pin")
	addCmd.Flags().StringSliceVar(&dependsOn, "depends-on", nil, "Dependencies (app/stack format)")
	addCmd.Flags().StringVar(&org, "org", "", "Pulumi Cloud organization (enables fully qualified stack names)")
	addCmd.Flags().StringSliceVar(&bases, "bases", nil, "Base configs to apply (ordered)")
	addCmd.Flags().StringVar(&addEnv, "env", "", "Environment (defaults to active env or 'default')")
	stackCmd.AddCommand(addCmd)

	// stack list
	stackCmd.AddCommand(&cobra.Command{
		Use:               "list [app]",
		Aliases:           []string{"ls"},
		Short:             "List stacks",
		Args:              cobra.MaximumNArgs(1),
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

			var apps []config.App
			if len(args) == 1 {
				app, err := cfg.FindApp(args[0])
				if err != nil {
					return err
				}
				apps = []config.App{*app}
			} else {
				apps = cfg.Apps
			}

			filterEnv := resolveEnv()
			first := true
			for _, app := range apps {
				var stacks []config.Stack
				for _, st := range app.Stacks {
					if filterEnv != "" && st.Env != filterEnv {
						continue
					}
					stacks = append(stacks, st)
				}
				if len(stacks) == 0 {
					continue
				}

				if !first {
					fmt.Println()
				}
				first = false
				stackName.Println(app.Name)
				stackMeta.Printf("  repo: %s\n", app.Repo)
				if app.Path != "." {
					stackMeta.Printf("  path: %s\n", app.Path)
				}
				fmt.Println()
				for _, st := range stacks {
					ref := st.Branch
					if st.Ref != "" {
						ref = st.Ref
					}
					if ref == "" {
						ref = "(default)"
					}
					fmt.Print("  ")
					stackName.Print(st.Name)
					fmt.Print("  ")
					stackRef.Print(ref)
					if st.Env != "" && st.Env != "default" {
						stackMeta.Printf("  env:%s", st.Env)
					}
					if st.Org != "" {
						stackMeta.Printf("  org:%s", st.Org)
					}
					if st.Project != "" {
						stackMeta.Printf("  project:%s", st.Project)
					}
					if len(st.DependsOn) > 0 {
						stackMeta.Printf("  deps:%s", strings.Join(st.DependsOn, ","))
					}
					if len(st.Bases) > 0 {
						stackMeta.Printf("  bases:%s", strings.Join(st.Bases, ","))
					}
					fmt.Println()
				}
			}
			return nil
		},
	})

	// stack cp
	stackCmd.AddCommand(&cobra.Command{
		Use:               "cp <src-app/stack> <dst-app/stack>",
		Aliases:           []string{"copy"},
		Short:             "Copy a stack to another app (config entry + Pulumi config file)",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeAppStacks,
		RunE: func(cmd *cobra.Command, args []string) error {
			srcAppName, srcStackName, err := splitAppStack(args[0])
			if err != nil {
				return err
			}
			dstAppName, dstStackName, err := splitAppStack(args[1])
			if err != nil {
				return err
			}

			s, err := getStore()
			if err != nil {
				return err
			}

			cfg, err := s.LoadConfig()
			if err != nil {
				return err
			}

			srcApp, err := cfg.FindApp(srcAppName)
			if err != nil {
				return err
			}
			srcStack, err := srcApp.FindStack(srcStackName)
			if err != nil {
				return err
			}

			dstApp, err := cfg.FindApp(dstAppName)
			if err != nil {
				return err
			}
			if _, err := dstApp.FindStack(dstStackName); err == nil {
				return fmt.Errorf("stack %q already exists in app %q", dstStackName, dstApp.Name)
			}

			// Copy stack definition with new name
			newStack := *srcStack
			newStack.Name = dstStackName
			dstApp.Stacks = append(dstApp.Stacks, newStack)

			if err := s.SaveConfig(cfg); err != nil {
				return err
			}

			// Copy Pulumi config file via store if it exists
			if data, readErr := s.ReadStackConfig(srcApp.Name, srcStack.Name); readErr == nil && data != nil {
				if writeErr := s.WriteStackConfig(dstApp.Name, dstStackName, data); writeErr != nil {
					return fmt.Errorf("copying stack config: %w", writeErr)
				}
				fmt.Printf("Copied stack %s → %s (config entry + Pulumi config file)\n", args[0], args[1])
			} else {
				fmt.Printf("Copied stack %s → %s (config entry only, no Pulumi config file found)\n", args[0], args[1])
			}

			return nil
		},
	})

	// stack edit
	stackCmd.AddCommand(&cobra.Command{
		Use:               "edit <app/stack>",
		Short:             "Open the stack definition in your editor",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeAppStacks,
		RunE: func(cmd *cobra.Command, args []string) error {
			appName, stackName, err := splitAppStack(args[0])
			if err != nil {
				return err
			}

			s, err := getStore()
			if err != nil {
				return err
			}

			path, err := s.StackFilePath(appName, stackName)
			if err != nil {
				return err
			}

			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}

			c := exec.Command(editor, path)
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			return c.Run()
		},
	})

	// stack rename
	stackCmd.AddCommand(&cobra.Command{
		Use:               "rename <app/stack> <new-name>",
		Aliases:           []string{"mv"},
		Short:             "Rename a stack",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeAppStacks,
		RunE: func(cmd *cobra.Command, args []string) error {
			appName, oldStackName, err := splitAppStack(args[0])
			if err != nil {
				return err
			}
			newName := args[1]

			s, err := getStore()
			if err != nil {
				return err
			}

			cfg, err := s.LoadConfig()
			if err != nil {
				return err
			}

			app, err := cfg.FindApp(appName)
			if err != nil {
				return err
			}

			stack, err := app.FindStack(oldStackName)
			if err != nil {
				return err
			}

			if _, err := app.FindStack(newName); err == nil {
				return fmt.Errorf("stack %q already exists in app %q", newName, app.Name)
			}

			oldName := stack.Name

			// Update dependsOn references across all apps
			oldKey := app.Name + "/" + oldName
			newKey := app.Name + "/" + newName
			for i := range cfg.Apps {
				for j := range cfg.Apps[i].Stacks {
					for k, dep := range cfg.Apps[i].Stacks[j].DependsOn {
						if dep == oldKey {
							cfg.Apps[i].Stacks[j].DependsOn[k] = newKey
						}
					}
				}
			}

			stack.Name = newName

			// Rename the stack file before SaveConfig so the config section is preserved
			if oldPath, err := s.StackFilePath(appName, oldName); err == nil {
				newPath := strings.TrimSuffix(oldPath, oldName+".yaml") + newName + ".yaml"
				os.Rename(oldPath, newPath)
			}

			if err := s.SaveConfig(cfg); err != nil {
				return err
			}

			// Rename Pulumi stack config file in store (legacy, if separate)
			if data, readErr := s.ReadStackConfig(app.Name, oldName); readErr == nil && data != nil {
				if writeErr := s.WriteStackConfig(app.Name, newName, data); writeErr != nil {
					return fmt.Errorf("renaming stack config: %w", writeErr)
				}
			}

			fmt.Printf("Renamed stack %s/%s → %s/%s\n", app.Name, oldName, app.Name, newName)
			return nil
		},
	})

	// stack move
	stackCmd.AddCommand(&cobra.Command{
		Use:               "move <app/stack> <env>",
		Short:             "Move a stack to another environment",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeAppStacks,
		RunE: func(cmd *cobra.Command, args []string) error {
			appName, stackName, err := splitAppStack(args[0])
			if err != nil {
				return err
			}

			s, err := getStore()
			if err != nil {
				return err
			}

			cfg, err := s.LoadConfig()
			if err != nil {
				return err
			}

			app, err := cfg.FindApp(appName)
			if err != nil {
				return err
			}

			stack, err := app.FindStack(stackName)
			if err != nil {
				return err
			}

			stack.Env = args[1]
			if err := s.SaveConfig(cfg); err != nil {
				return err
			}

			fmt.Printf("Moved %s to environment %q\n", args[0], args[1])
			return nil
		},
	})

	// stack remove
	stackCmd.AddCommand(&cobra.Command{
		Use:               "remove <app/stack>",
		Aliases:           []string{"rm"},
		Short:             "Remove a stack from an app",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeAppStacks,
		RunE: func(cmd *cobra.Command, args []string) error {
			appName, stackName, err := splitAppStack(args[0])
			if err != nil {
				return err
			}

			s, err := getStore()
			if err != nil {
				return err
			}

			cfg, err := s.LoadConfig()
			if err != nil {
				return err
			}

			app, err := cfg.FindApp(appName)
			if err != nil {
				return err
			}

			idx := -1
			for i, st := range app.Stacks {
				if st.Name == stackName {
					idx = i
					break
				}
			}
			if idx == -1 {
				return fmt.Errorf("stack %q not found in app %q", stackName, app.Name)
			}

			app.Stacks = append(app.Stacks[:idx], app.Stacks[idx+1:]...)
			if err := s.SaveConfig(cfg); err != nil {
				return err
			}
			fmt.Printf("Removed stack %s\n", args[0])
			return nil
		},
	})
}
