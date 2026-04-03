package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/gfyrag/plr/internal/git"
	pulumibridge "github.com/gfyrag/plr/internal/pulumi"
	"github.com/spf13/cobra"
)

var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "Manage Pulumi stack state",
}

func init() {
	rootCmd.AddCommand(stateCmd)

	// state delete
	var force bool
	deleteCmd := &cobra.Command{
		Use:   "delete <app/stack> <urn>",
		Short: "Remove a resource from the stack state",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := getStore()
			if err != nil {
				return err
			}

			app, stack, err := resolveAppStack(s, args[0])
			if err != nil {
				return err
			}
			urn := args[1]

			workDir, err := git.WorkDir(app)
			if err != nil {
				return err
			}

			ps, err := pulumibridge.GetStack(cmd.Context(), stack, workDir)
			if err != nil {
				return err
			}

			deployment, err := ps.Export(cmd.Context())
			if err != nil {
				return fmt.Errorf("exporting state: %w", err)
			}

			var state map[string]any
			if err := json.Unmarshal(deployment.Deployment, &state); err != nil {
				return fmt.Errorf("parsing state: %w", err)
			}

			// Remove from resources
			modified := false
			if resources, ok := state["resources"].([]any); ok {
				var filtered []any
				for _, r := range resources {
					res, ok := r.(map[string]any)
					if !ok {
						filtered = append(filtered, r)
						continue
					}
					if res["urn"] == urn {
						modified = true
						fmt.Printf("Removed resource: %s\n", urn)
						continue
					}
					// Also remove any resource that has this URN as parent
					if !force {
						filtered = append(filtered, r)
					} else if res["parent"] == urn {
						modified = true
						fmt.Printf("Removed child resource: %s\n", res["urn"])
					} else {
						filtered = append(filtered, r)
					}
				}
				state["resources"] = filtered
			}

			// Clear pending operations referencing this URN
			if pending, ok := state["pending_operations"].([]any); ok {
				var filtered []any
				for _, p := range pending {
					op, ok := p.(map[string]any)
					if !ok {
						filtered = append(filtered, p)
						continue
					}
					if res, ok := op["resource"].(map[string]any); ok && res["urn"] == urn {
						modified = true
						fmt.Printf("Cleared pending operation on: %s\n", urn)
						continue
					}
					filtered = append(filtered, p)
				}
				state["pending_operations"] = filtered
			}

			if !modified {
				return fmt.Errorf("URN %q not found in state", urn)
			}

			data, err := json.Marshal(state)
			if err != nil {
				return fmt.Errorf("serializing state: %w", err)
			}
			deployment.Deployment = data

			if err := ps.Import(cmd.Context(), deployment); err != nil {
				return fmt.Errorf("importing state: %w", err)
			}

			fmt.Println("State updated successfully.")
			return nil
		},
	}
	deleteCmd.Flags().BoolVarP(&force, "force", "f", false, "Also remove child resources")
	stateCmd.AddCommand(deleteCmd)

	// state clear-pending
	stateCmd.AddCommand(&cobra.Command{
		Use:   "clear-pending <app/stack>",
		Short: "Clear all pending operations from the stack state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := getStore()
			if err != nil {
				return err
			}

			app, stack, err := resolveAppStack(s, args[0])
			if err != nil {
				return err
			}

			workDir, err := git.WorkDir(app)
			if err != nil {
				return err
			}

			ps, err := pulumibridge.GetStack(cmd.Context(), stack, workDir)
			if err != nil {
				return err
			}

			deployment, err := ps.Export(cmd.Context())
			if err != nil {
				return fmt.Errorf("exporting state: %w", err)
			}

			var state map[string]any
			if err := json.Unmarshal(deployment.Deployment, &state); err != nil {
				return fmt.Errorf("parsing state: %w", err)
			}

			pending, ok := state["pending_operations"].([]any)
			if !ok || len(pending) == 0 {
				fmt.Println("No pending operations.")
				return nil
			}

			// Collect URNs of pending CREATEs — these resources must be
			// removed from the resources list so Pulumi doesn't try to
			// re-create something that may already exist in the provider.
			pendingCreateURNs := make(map[string]bool)
			for _, p := range pending {
				if op, ok := p.(map[string]any); ok {
					opType, _ := op["type"].(string)
					if res, ok := op["resource"].(map[string]any); ok {
						urn, _ := res["urn"].(string)
						fmt.Printf("Clearing: %s (%s)\n", urn, opType)
						if opType == "creating" {
							pendingCreateURNs[urn] = true
						}
					}
				}
			}

			// Remove resources that were interrupted mid-create
			if len(pendingCreateURNs) > 0 {
				if resources, ok := state["resources"].([]any); ok {
					var filtered []any
					for _, r := range resources {
						res, ok := r.(map[string]any)
						if ok {
							if urn, ok := res["urn"].(string); ok && pendingCreateURNs[urn] {
								fmt.Printf("Removed incomplete resource: %s\n", urn)
								continue
							}
						}
						filtered = append(filtered, r)
					}
					state["resources"] = filtered
				}
			}

			state["pending_operations"] = []any{}

			data, err := json.Marshal(state)
			if err != nil {
				return fmt.Errorf("serializing state: %w", err)
			}
			deployment.Deployment = data

			if err := ps.Import(cmd.Context(), deployment); err != nil {
				return fmt.Errorf("importing state: %w", err)
			}

			fmt.Printf("Cleared %d pending operation(s).\n", len(pending))
			return nil
		},
	})
}
