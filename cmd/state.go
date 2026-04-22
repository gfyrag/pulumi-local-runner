package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fatih/color"
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

	// state list
	stateCmd.AddCommand(&cobra.Command{
		Use:               "list <app/stack>",
		Aliases:           []string{"ls"},
		Short:             "List resources in the stack state",
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

			git.EnsurePassphrase()

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

			urnColor := color.New(color.FgCyan)
			typeColor := color.New(color.FgYellow)
			pendingColor := color.New(color.Bold, color.FgRed)
			dimColor := color.New(color.Faint)

			// Show pending operations
			if pending, ok := state["pending_operations"].([]any); ok && len(pending) > 0 {
				pendingColor.Printf("Pending operations (%d):\n", len(pending))
				for _, p := range pending {
					op, _ := p.(map[string]any)
					opType, _ := op["type"].(string)
					if res, ok := op["resource"].(map[string]any); ok {
						urn, _ := res["urn"].(string)
						fmt.Printf("  %s  ", opType)
						urnColor.Println(urn)
					}
				}
				fmt.Println()
			}

			// Show resources
			resources, _ := state["resources"].([]any)
			fmt.Printf("Resources (%d):\n", len(resources))
			for _, r := range resources {
				res, ok := r.(map[string]any)
				if !ok {
					continue
				}
				urn, _ := res["urn"].(string)
				resType, _ := res["type"].(string)

				// Extract short name from URN (last segment after ::)
				shortName := urn
				if idx := strings.LastIndex(urn, "::"); idx >= 0 {
					shortName = urn[idx+2:]
				}

				// Show dependencies
				var deps []string
				if d, ok := res["dependencies"].([]any); ok {
					for _, dep := range d {
						if depStr, ok := dep.(string); ok {
							if idx := strings.LastIndex(depStr, "::"); idx >= 0 {
								deps = append(deps, depStr[idx+2:])
							}
						}
					}
				}

				typeColor.Printf("  %-50s ", resType)
				fmt.Print(shortName)
				if len(deps) > 0 {
					dimColor.Printf("  deps:[%s]", strings.Join(deps, ", "))
				}
				fmt.Println()
				dimColor.Printf("    %s\n", urn)
			}

			return nil
		},
	})

	// state delete
	var force bool
	deleteCmd := &cobra.Command{
		Use:               "delete <app/stack> <urn>",
		Short:             "Remove a resource from the stack state",
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

			// Collect all URNs to remove (cascade: children, provider dependents)
			toRemove := map[string]bool{urn: true}
			modified := false

			if resources, ok := state["resources"].([]any); ok {
				// If --force, also remove children
				if force {
					for _, r := range resources {
						res, _ := r.(map[string]any)
						if res == nil {
							continue
						}
						if res["parent"] == urn {
							resURN, _ := res["urn"].(string)
							toRemove[resURN] = true
						}
					}
				}

				// Also remove resources whose provider references a removed URN
				// (provider field is "urn::id" format)
				changed := true
				for changed {
					changed = false
					for _, r := range resources {
						res, _ := r.(map[string]any)
						if res == nil {
							continue
						}
						resURN, _ := res["urn"].(string)
						if toRemove[resURN] {
							continue
						}
						if provider, ok := res["provider"].(string); ok {
							for removedURN := range toRemove {
								if strings.HasPrefix(provider, removedURN+"::") {
									toRemove[resURN] = true
									changed = true
									break
								}
							}
						}
					}
				}

				// Filter resources
				var filtered []any
				for _, r := range resources {
					res, _ := r.(map[string]any)
					if res == nil {
						filtered = append(filtered, r)
						continue
					}
					resURN, _ := res["urn"].(string)
					if toRemove[resURN] {
						modified = true
						fmt.Printf("Removed resource: %s\n", resURN)
						continue
					}
					filtered = append(filtered, r)
				}

				// Clean up dependencies and provider references to removed URNs
				for _, r := range filtered {
					res, ok := r.(map[string]any)
					if !ok {
						continue
					}
					if deps, ok := res["dependencies"].([]any); ok {
						var cleanDeps []any
						for _, d := range deps {
							if dStr, ok := d.(string); ok && !toRemove[dStr] {
								cleanDeps = append(cleanDeps, d)
							}
						}
						res["dependencies"] = cleanDeps
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
		Use:               "clear-pending <app/stack>",
		Short:             "Clear all pending operations from the stack state",
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
