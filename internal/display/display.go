// Package display handles terminal output formatting and JSON serialization.
package display

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/handy-sun/githand/internal/i18n"
	"github.com/handy-sun/githand/internal/status"
)

// Status prints repo statuses to stdout.
func Status(results []status.RepoStatus, asJSON, showRemote bool) error {
	if asJSON {
		return statusJSON(results)
	}
	return statusTable(results, showRemote)
}

func statusTable(results []status.RepoStatus, showRemote bool) error {
	if len(results) == 0 {
		fmt.Println(i18n.T("display.no_repos"))
		return nil
	}

	header := strings.Split(i18n.T("display.header"), "\t")
	if showRemote {
		header = append(header, i18n.T("display.remote"))
	}
	rows := [][]string{header}
	for _, s := range results {
		state := i18n.T("display.clean")
		if s.Dirty {
			state = i18n.T("display.dirty")
		}
		branch := s.Branch
		if s.Detached {
			short := s.Commit
			if len(short) > 8 {
				short = short[:8]
			}
			branch = fmt.Sprintf("(%s)", short)
		}
		row := []string{
			s.Repo.Name,
			branch,
			state,
			fmt.Sprint(s.Ahead),
			fmt.Sprint(s.Behind),
			fmt.Sprint(s.StashCount),
		}
		if showRemote {
			row = append(row, status.PrimarySource(s.Remotes))
		}
		rows = append(rows, row)
	}
	return writeTable(os.Stdout, rows)
}

func statusJSON(results []status.RepoStatus) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(results)
}

func writeTable(w io.Writer, rows [][]string) error {
	widths := make([]int, 0)
	for _, row := range rows {
		for col, cell := range row {
			if col == len(widths) {
				widths = append(widths, 0)
			}
			if width := displayWidth(cell); width > widths[col] {
				widths[col] = width
			}
		}
	}

	for _, row := range rows {
		for col, cell := range row {
			if col > 0 {
				if _, err := fmt.Fprint(w, "  "); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprint(w, cell); err != nil {
				return err
			}
			if col < len(row)-1 {
				padding := widths[col] - displayWidth(cell)
				if padding > 0 {
					if _, err := fmt.Fprint(w, strings.Repeat(" ", padding)); err != nil {
						return err
					}
				}
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	return nil
}

func displayWidth(s string) int {
	width := 0
	for _, r := range s {
		switch {
		case r == 0:
		case r < 32 || (r >= 0x7f && r < 0xa0):
		case unicode.Is(unicode.Mn, r):
		case isWideRune(r):
			width += 2
		default:
			width++
		}
	}
	return width
}

func isWideRune(r rune) bool {
	return (r >= 0x1100 && r <= 0x115f) ||
		r == 0x2329 || r == 0x232a ||
		(r >= 0x2e80 && r <= 0xa4cf && r != 0x303f) ||
		(r >= 0xac00 && r <= 0xd7a3) ||
		(r >= 0xf900 && r <= 0xfaff) ||
		(r >= 0xfe10 && r <= 0xfe19) ||
		(r >= 0xfe30 && r <= 0xfe6f) ||
		(r >= 0xff00 && r <= 0xff60) ||
		(r >= 0xffe0 && r <= 0xffe6) ||
		(r >= 0x1f300 && r <= 0x1faff)
}
