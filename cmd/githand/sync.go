package main

import (
	"fmt"

	"github.com/handy-sun/githand/internal/i18n"
	synccmd "github.com/handy-sun/githand/internal/sync"
	"github.com/spf13/cobra"
)

var (
	syncGroup  string
	syncRemote string
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: i18n.T("sync.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, cfg, reg := mustLoadConfig()

		workers := cfg.Status.Workers
		results := synccmd.Run(&reg, syncGroup, syncRemote, workers)

		for _, r := range results {
			switch r.Status {
			case "updated":
				fmt.Printf("  %-20s  %s\n", r.Name, r.Detail)
			case "up-to-date":
				fmt.Printf("  %-20s  %s (up to date)\n", r.Name, r.Detail)
			case "fetched":
				fmt.Printf("  %-20s  %s\n", r.Name, r.Detail)
			case "skipped":
				fmt.Printf("  %-20s  skipped: %s\n", r.Name, r.Detail)
			case "error":
				fmt.Printf("  %-20s  error: %s\n", r.Name, r.Detail)
			}
		}

		// Summary
		updated, failed := 0, 0
		for _, r := range results {
			switch r.Status {
			case "updated", "fetched":
				updated++
			case "error":
				failed++
			}
		}
		fmt.Println()
		fmt.Println(i18n.Tf("sync.summary", len(results), updated, failed))
		return nil
	},
}

func init() {
	syncCmd.Flags().StringVar(&syncGroup, "group", "", i18n.T("sync.flag.group"))
	syncCmd.Flags().StringVar(&syncRemote, "remote", "", i18n.T("sync.flag.remote"))
}
