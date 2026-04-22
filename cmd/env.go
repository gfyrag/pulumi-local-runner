package cmd

import (
	"fmt"
	"sort"

	"github.com/fatih/color"
	"github.com/gfyrag/plr/internal/config"
	"github.com/spf13/cobra"
)

var (
	envName    = color.New(color.Bold, color.FgCyan)
	envActive  = color.New(color.Bold, color.FgGreen)
	envTarget  = color.New(color.Faint)
)

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage environments",
}

func init() {
	rootCmd.AddCommand(envCmd)

	// env list
	envCmd.AddCommand(&cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all environments",
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

			activeEnv, _ := s.ReadActiveEnv()
			envs := collectEnvs(cfg)

			for _, env := range envs {
				if env == activeEnv {
					envActive.Printf("* %s\n", env)
				} else {
					fmt.Printf("  %s\n", env)
				}
			}
			return nil
		},
	})

	// env current
	envCmd.AddCommand(&cobra.Command{
		Use:   "current",
		Short: "Show the active environment",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := getStore()
			if err != nil {
				return err
			}

			env, err := s.ReadActiveEnv()
			if err != nil {
				return err
			}

			if env == "" {
				fmt.Println("No active environment set.")
			} else {
				fmt.Println(env)
			}
			return nil
		},
	})

	// env use
	envCmd.AddCommand(&cobra.Command{
		Use:   "use <name>",
		Short: "Set the active environment",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: completeEnvs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := getStore()
			if err != nil {
				return err
			}

			if err := s.WriteActiveEnv(args[0]); err != nil {
				return err
			}

			fmt.Printf("Active environment set to %q\n", args[0])
			return nil
		},
	})

	// env show
	envCmd.AddCommand(&cobra.Command{
		Use:   "show [name]",
		Short: "Show stacks in an environment",
		Args:  cobra.MaximumNArgs(1),
		ValidArgsFunction: completeEnvs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := getStore()
			if err != nil {
				return err
			}

			cfg, err := s.LoadConfig()
			if err != nil {
				return err
			}

			env := ""
			if len(args) == 1 {
				env = args[0]
			} else {
				env, _ = s.ReadActiveEnv()
				if env == "" {
					return fmt.Errorf("no environment specified and no active environment set")
				}
			}

			envName.Println(env)
			for _, app := range cfg.Apps {
				for _, st := range app.Stacks {
					if st.Env == env {
						envTarget.Printf("  %s/%s\n", app.Name, st.Name)
					}
				}
			}
			return nil
		},
	})

	// env rename
	envCmd.AddCommand(&cobra.Command{
		Use:   "rename <old> <new>",
		Short: "Rename an environment across all stacks",
		Args:  cobra.ExactArgs(2),
		ValidArgsFunction: completeEnvs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := getStore()
			if err != nil {
				return err
			}

			cfg, err := s.LoadConfig()
			if err != nil {
				return err
			}

			oldName, newName := args[0], args[1]
			count := 0
			for i := range cfg.Apps {
				for j := range cfg.Apps[i].Stacks {
					if cfg.Apps[i].Stacks[j].Env == oldName {
						cfg.Apps[i].Stacks[j].Env = newName
						count++
					}
				}
			}

			if count == 0 {
				return fmt.Errorf("no stacks found in environment %q", oldName)
			}

			if err := s.SaveConfig(cfg); err != nil {
				return err
			}

			// Update active env if it was renamed
			activeEnv, _ := s.ReadActiveEnv()
			if activeEnv == oldName {
				s.WriteActiveEnv(newName)
			}

			fmt.Printf("Renamed environment %q to %q (%d stacks)\n", oldName, newName, count)
			return nil
		},
	})
}

// collectEnvs returns sorted unique environment names from all stacks.
func collectEnvs(cfg *config.Config) []string {
	seen := make(map[string]bool)
	for _, app := range cfg.Apps {
		for _, st := range app.Stacks {
			if st.Env != "" {
				seen[st.Env] = true
			}
		}
	}
	var envs []string
	for e := range seen {
		envs = append(envs, e)
	}
	sort.Strings(envs)
	return envs
}

// completeEnvs provides shell completion for environment names.
func completeEnvs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	s, err := getStore()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	cfg, err := s.LoadConfig()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return collectEnvs(cfg), cobra.ShellCompDirectiveNoFileComp
}
