package main

import (
	"fmt"

	"github.com/handy-sun/githand/internal/display"
	"github.com/handy-sun/githand/internal/status"
	"github.com/spf13/cobra"
)

var (
	statusFilter string
	statusGroup  string
	statusUser   string
	statusJSON   bool
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of all registered repos",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, cfg, reg := mustLoadConfig()

		// CLI flags override config
		asJSON := statusJSON
		if !cmd.Flags().Changed("json") {
			asJSON = cfg.Status.JSON
		}

		workers := cfg.Status.Workers
		results, err := status.Collect(&reg, workers)
		if err != nil {
			return fmt.Errorf("collect status: %w", err)
		}

		// apply static filters first
		if statusGroup != "" {
			results = status.FilterByGroup(results, &reg, statusGroup)
		}
		if statusUser != "" {
			results = status.FilterByUser(results, statusUser)
		}

		// apply dynamic filters that need git data
		if statusFilter != "" {
			results = status.FilterByFlag(results, statusFilter)
		}

		return display.Status(results, asJSON)
	},
}

func init() {
	statusCmd.Flags().StringVar(&statusFilter, "filter", "", "filter: dirty, ahead, stash, detached")
	statusCmd.Flags().StringVar(&statusGroup, "group", "", "filter by group name")
	statusCmd.Flags().StringVar(&statusUser, "user", "", "filter by remote URL owner")
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "machine-readable JSON output")
}
