package cmd

import (
	"fmt"
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
	var branch, ref, org string
	var dependsOn []string
	addCmd := &cobra.Command{
		Use:   "add <app> <name>",
		Short: "Add a stack to an app",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
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

			stack := config.Stack{
				Name:      name,
				Branch:    branch,
				Ref:       ref,
				DependsOn: dependsOn,
				Org:       org,
			}

			app.Stacks = append(app.Stacks, stack)
			if err := config.Save(cfg); err != nil {
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
	stackCmd.AddCommand(addCmd)

	// stack list
	stackCmd.AddCommand(&cobra.Command{
		Use:   "list [app]",
		Short: "List stacks",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
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

			for _, app := range apps {
				for _, s := range app.Stacks {
					ref := s.Branch
					if s.Ref != "" {
						ref = s.Ref
					}
					if ref == "" {
						ref = "(default)"
					}
					stackName.Printf("%s/%s", app.Name, s.Name)
					fmt.Print("  ")
					stackRef.Print(ref)
					if s.Org != "" {
						stackMeta.Printf("  org:%s", s.Org)
					}
					if len(s.DependsOn) > 0 {
						stackMeta.Printf("  deps:%s", strings.Join(s.DependsOn, ","))
					}
					fmt.Println()
				}
			}
			return nil
		},
	})

	// stack remove
	stackCmd.AddCommand(&cobra.Command{
		Use:   "remove <app/stack>",
		Short: "Remove a stack from an app",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parts := strings.SplitN(args[0], "/", 2)
			if len(parts) != 2 {
				return fmt.Errorf("expected app/stack format, got %q", args[0])
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			app, err := cfg.FindApp(parts[0])
			if err != nil {
				return err
			}

			idx := -1
			for i, s := range app.Stacks {
				if s.Name == parts[1] {
					idx = i
					break
				}
			}
			if idx == -1 {
				return fmt.Errorf("stack %q not found in app %q", parts[1], app.Name)
			}

			app.Stacks = append(app.Stacks[:idx], app.Stacks[idx+1:]...)
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Printf("Removed stack %s\n", args[0])
			return nil
		},
	})
}
