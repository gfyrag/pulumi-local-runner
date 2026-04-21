package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var saltCmd = &cobra.Command{
	Use:   "salt",
	Short: "Manage the global encryption salt",
}

func init() {
	rootCmd.AddCommand(saltCmd)

	// salt show
	saltCmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Print the global encryption salt",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := getStore()
			if err != nil {
				return err
			}

			salt, err := s.ReadEncryptionSalt()
			if err != nil {
				return err
			}

			if salt == "" {
				fmt.Println("No global encryption salt configured.")
				return nil
			}

			fmt.Println(salt)
			return nil
		},
	})

	// salt set
	saltCmd.AddCommand(&cobra.Command{
		Use:   "set <salt>",
		Short: "Set the global encryption salt",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := getStore()
			if err != nil {
				return err
			}

			if err := s.WriteEncryptionSalt(args[0]); err != nil {
				return err
			}

			fmt.Println("Global encryption salt updated.")
			return nil
		},
	})

	// salt adopt — take the salt from an existing stack
	saltCmd.AddCommand(&cobra.Command{
		Use:               "adopt <app/stack>",
		Short:             "Adopt the encryption salt from an existing stack as the global salt",
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

			data, err := s.ReadStackConfig(app.Name, stack.Name)
			if err != nil {
				return err
			}
			if data == nil {
				return fmt.Errorf("stack %s/%s has no config", app.Name, stack.Name)
			}

			// Extract encryptionsalt from the Pulumi config (which is YAML)
			var parsed map[string]any
			if err := yaml.Unmarshal(data, &parsed); err != nil {
				return fmt.Errorf("parsing stack config: %w", err)
			}
			salt, _ := parsed["encryptionsalt"].(string)

			if salt == "" {
				return fmt.Errorf("stack %s/%s has no encryption salt", app.Name, stack.Name)
			}

			if err := s.WriteEncryptionSalt(salt); err != nil {
				return err
			}

			fmt.Printf("Adopted encryption salt from %s/%s\n", app.Name, stack.Name)
			return nil
		},
	})
}
