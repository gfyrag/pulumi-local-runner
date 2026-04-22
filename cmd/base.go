package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var baseCmd = &cobra.Command{
	Use:   "base",
	Short: "Manage base config templates",
}

func completeBases(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	s, err := getStore()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	names, err := s.ListBases()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}

func init() {
	rootCmd.AddCommand(baseCmd)

	// base list
	baseCmd.AddCommand(&cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all bases",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := getStore()
			if err != nil {
				return err
			}

			names, err := s.ListBases()
			if err != nil {
				return err
			}

			if len(names) == 0 {
				fmt.Println("No bases configured.")
				return nil
			}

			for _, name := range names {
				fmt.Println(name)
			}
			return nil
		},
	})

	// base show
	baseCmd.AddCommand(&cobra.Command{
		Use:               "show <name>",
		Short:             "Print a base config to stdout",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeBases,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := getStore()
			if err != nil {
				return err
			}

			data, err := s.ReadBaseConfig(args[0])
			if err != nil {
				return err
			}
			if data == nil {
				return fmt.Errorf("base %q not found", args[0])
			}

			fmt.Print(string(data))
			return nil
		},
	})

	// base edit
	baseCmd.AddCommand(&cobra.Command{
		Use:               "edit <name>",
		Short:             "Open a base config in your editor (creates if new)",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeBases,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := getStore()
			if err != nil {
				return err
			}

			name := args[0]
			data, err := s.ReadBaseConfig(name)
			if err != nil {
				return err
			}

			tmpDir, err := os.MkdirTemp("", "plr-base-edit-*")
			if err != nil {
				return err
			}
			defer os.RemoveAll(tmpDir)

			tmpFile := filepath.Join(tmpDir, fmt.Sprintf("%s.yaml", name))
			if data != nil {
				if err := os.WriteFile(tmpFile, data, 0o644); err != nil {
					return err
				}
			}

			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}

			c := exec.Command(editor, tmpFile)
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			if err := c.Run(); err != nil {
				return err
			}

			edited, err := os.ReadFile(tmpFile)
			if err != nil {
				return err
			}

			return s.WriteBaseConfig(name, edited)
		},
	})

	// base import
	baseCmd.AddCommand(&cobra.Command{
		Use:   "import <name> <path-to-yaml>",
		Short: "Import a YAML file as a base config",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := getStore()
			if err != nil {
				return err
			}

			data, err := os.ReadFile(args[1])
			if err != nil {
				return fmt.Errorf("reading %s: %w", args[1], err)
			}

			if err := s.WriteBaseConfig(args[0], data); err != nil {
				return err
			}

			fmt.Printf("Imported base %q\n", args[0])
			return nil
		},
	})

	// base set
	baseCmd.AddCommand(&cobra.Command{
		Use:               "set <name> <key> <value>",
		Short:             "Set a config value on a base",
		Args:              cobra.ExactArgs(3),
		ValidArgsFunction: completeBases,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := getStore()
			if err != nil {
				return err
			}

			name, key, value := args[0], args[1], args[2]

			data, err := s.ReadBaseConfig(name)
			if err != nil {
				return err
			}

			var base map[string]any
			if data != nil {
				if err := yaml.Unmarshal(data, &base); err != nil {
					return fmt.Errorf("parsing base: %w", err)
				}
			}
			if base == nil {
				base = make(map[string]any)
			}

			cfg, _ := base["config"].(map[string]any)
			if cfg == nil {
				cfg = make(map[string]any)
			}

			cfg[key] = value
			base["config"] = cfg
			out, err := yaml.Marshal(base)
			if err != nil {
				return err
			}

			return s.WriteBaseConfig(name, out)
		},
	})

	// base get
	baseCmd.AddCommand(&cobra.Command{
		Use:               "get <name> <key>",
		Short:             "Get a config value from a base",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeBases,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := getStore()
			if err != nil {
				return err
			}

			data, err := s.ReadBaseConfig(args[0])
			if err != nil {
				return err
			}
			if data == nil {
				return fmt.Errorf("base %q not found", args[0])
			}

			var base map[string]any
			if err := yaml.Unmarshal(data, &base); err != nil {
				return fmt.Errorf("parsing base: %w", err)
			}

			cfg, _ := base["config"].(map[string]any)
			val, ok := cfg[args[1]]
			if !ok {
				return fmt.Errorf("key %q not found in base %q", args[1], args[0])
			}

			fmt.Println(val)
			return nil
		},
	})

	// base rm-key
	baseCmd.AddCommand(&cobra.Command{
		Use:               "rm-key <name> <key>",
		Short:             "Remove a config key from a base",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeBases,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := getStore()
			if err != nil {
				return err
			}

			data, err := s.ReadBaseConfig(args[0])
			if err != nil {
				return err
			}
			if data == nil {
				return fmt.Errorf("base %q not found", args[0])
			}

			var base map[string]any
			if err := yaml.Unmarshal(data, &base); err != nil {
				return fmt.Errorf("parsing base: %w", err)
			}

			cfg, _ := base["config"].(map[string]any)
			if _, ok := cfg[args[1]]; !ok {
				return fmt.Errorf("key %q not found in base %q", args[1], args[0])
			}

			delete(cfg, args[1])
			base["config"] = cfg

			out, err := yaml.Marshal(base)
			if err != nil {
				return err
			}

			return s.WriteBaseConfig(args[0], out)
		},
	})

	// base rm
	baseCmd.AddCommand(&cobra.Command{
		Use:               "rm <name>",
		Aliases:           []string{"remove"},
		Short:             "Delete a base config",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeBases,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := getStore()
			if err != nil {
				return err
			}

			if err := s.DeleteBaseConfig(args[0]); err != nil {
				return err
			}

			fmt.Printf("Removed base %q\n", args[0])
			return nil
		},
	})
}
