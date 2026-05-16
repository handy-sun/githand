package main

import (
	"fmt"

	"github.com/handy-sun/githand/internal/config"
	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Remove a repo from the registry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		dir, _, reg := mustLoadConfig()

		if !reg.RemoveRepo(name) {
			return fmt.Errorf("repo %q not found in registry", name)
		}

		// also remove from groups
		for g, members := range reg.Groups {
			for i, m := range members {
				if m == name {
					reg.Groups[g] = append(members[:i], members[i+1:]...)
					break
				}
			}
			if len(reg.Groups[g]) == 0 {
				delete(reg.Groups, g)
			}
		}

		if err := config.SaveRegistry(dir, reg); err != nil {
			return fmt.Errorf("save registry: %w", err)
		}

		fmt.Printf("Removed %q from registry.\n", name)
		return nil
	},
}
