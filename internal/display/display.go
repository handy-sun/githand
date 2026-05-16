// Package display handles terminal output formatting and JSON serialization.
package display

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/handy-sun/githand/internal/status"
)

// Status prints repo statuses to stdout.
func Status(results []status.RepoStatus, asJSON bool) error {
	if asJSON {
		return statusJSON(results)
	}
	return statusTable(results)
}

func statusTable(results []status.RepoStatus) error {
	if len(results) == 0 {
		fmt.Println("No repositories registered.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "REPO\tBRANCH\tSTATUS\tAHEAD\tBEHIND\tSTASH")
	for _, s := range results {
		state := "clean"
		if s.Dirty {
			state = "dirty"
		}
		branch := s.Branch
		if s.Detached {
			short := s.Commit
			if len(short) > 8 {
				short = short[:8]
			}
			branch = fmt.Sprintf("(%s)", short)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\t%d\n",
			s.Repo.Name, branch, state, s.Ahead, s.Behind, s.StashCount)
	}
	return w.Flush()
}

func statusJSON(results []status.RepoStatus) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(results)
}
