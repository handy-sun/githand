package main

import (
	"fmt"
	"strings"

	"github.com/handy-sun/githand/internal/display"
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
		updated, failed := 0, 0

		results := synccmd.Run(&reg, syncGroup, syncRemote, workers, func(r synccmd.Result) {
			// Repo name header
			switch r.Status {
			case "updated":
				fmt.Println(display.Green(r.Name))
			case "error":
				fmt.Println(display.Redf("%s (error)", r.Name))
			default:
				fmt.Println(r.Name)
			}

			// Git pull output (indented)
			if r.GitOut != "" {
				for _, line := range strings.Split(r.GitOut, "\n") {
					if s := strings.TrimSpace(line); s != "" {
						fmt.Printf("    %s\n", s)
					}
				}
			}

			// Result line
			switch r.Status {
			case "updated":
				if r.OldHash != "" && r.NewHash != "" {
					fmt.Printf("    %s\n", display.Greenf("%s..%s (%s)", r.OldHash, r.NewHash, r.Detail))
				} else {
					fmt.Printf("    %s\n", display.Green(r.Detail))
				}
				updated++
			case "up-to-date":
				// No extra line — git output already says it
			case "fetched":
				fmt.Printf("    fetched (%s)\n", r.Detail)
				updated++
			case "skipped":
				fmt.Printf("    skipped: %s\n", r.Detail)
			case "error":
				if r.Detail != "" {
					fmt.Printf("    %s\n", display.Red(r.Detail))
				}
				failed++
			}

			fmt.Println()
		})

		// Summary
		fmt.Println(i18n.Tf("sync.summary", len(results), updated, failed))
		return nil
	},
}

func init() {
	syncCmd.Flags().StringVar(&syncGroup, "group", "", i18n.T("sync.flag.group"))
	syncCmd.Flags().StringVar(&syncRemote, "remote", "", i18n.T("sync.flag.remote"))
}
