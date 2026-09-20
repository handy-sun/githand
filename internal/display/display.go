// Package display handles terminal output formatting and JSON serialization.
package display

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/handy-sun/githand/internal/config"
	"github.com/handy-sun/githand/internal/i18n"
	"github.com/handy-sun/githand/internal/status"
)

// Status prints repo statuses to stdout. When the registry spans several
// workspace roots, table output groups repos per root under a banner.
func Status(reg *config.Registry, results []status.RepoStatus, asJSON, showRemote bool) error {
	if asJSON {
		return statusJSON(results)
	}
	return statusTable(reg, results, showRemote)
}

// statusSection is one root's banner plus its table rows (header first).
type statusSection struct {
	label string
	rows  [][]string
}

func statusTable(reg *config.Registry, results []status.RepoStatus, showRemote bool) error {
	if len(results) == 0 {
		fmt.Println(i18n.T("display.no_repos"))
		return nil
	}

	sections := groupByRoot(reg, results, showRemote)

	// single root (or no registry): keep the flat, banner-free layout
	if len(sections) == 1 {
		return writeTable(os.Stdout, sections[0].rows)
	}

	// shared column widths so every root's table lines up
	widths := make([]int, 0)
	for _, section := range sections {
		for _, row := range section.rows {
			for col, cell := range row {
				if col == len(widths) {
					widths = append(widths, 0)
				}
				if width := displayWidth(cell); width > widths[col] {
					widths[col] = width
				}
			}
		}
	}

	out := os.Stdout
	for i, section := range sections {
		if i > 0 {
			if _, err := fmt.Fprintln(out); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(out, "********** %s **********\n", section.label); err != nil {
			return err
		}
		if err := writeTableWidths(out, section.rows, widths); err != nil {
			return err
		}
	}
	return nil
}

// groupByRoot splits results into per-root sections ordered by the registry's
// root order (primary first). Repos outside every registered root form a
// trailing "other" section. Empty roots are skipped.
func groupByRoot(reg *config.Registry, results []status.RepoStatus, showRemote bool) []statusSection {
	header := strings.Split(i18n.T("display.header"), "\t")
	if showRemote {
		header = append(header, i18n.T("display.remote"))
	}

	section := func(label string, group []status.RepoStatus) statusSection {
		rows := [][]string{header}
		for _, s := range group {
			rows = append(rows, statusRow(s, showRemote))
		}
		return statusSection{label: label, rows: rows}
	}

	if reg == nil {
		return []statusSection{section("", results)}
	}

	byRoot := make(map[string][]status.RepoStatus)
	var other []status.RepoStatus
	for _, s := range results {
		if anchor, _, ok := reg.AnchorFor(s.Repo.Path); ok {
			byRoot[anchor] = append(byRoot[anchor], s)
		} else {
			other = append(other, s)
		}
	}

	var sections []statusSection
	roots := reg.Roots()
	labels := rootLabels(roots)
	for _, root := range roots {
		if len(byRoot[root]) == 0 {
			continue
		}
		sections = append(sections, section(labels[root], byRoot[root]))
	}
	if len(other) > 0 {
		sections = append(sections, section(i18n.T("display.other_root"), other))
	}
	if len(sections) == 0 {
		return []statusSection{section("", results)}
	}
	return sections
}

// rootLabels names each root by its base directory; roots sharing a basename
// fall back to the full path so banners stay unambiguous.
func rootLabels(roots []string) map[string]string {
	counts := make(map[string]int, len(roots))
	for _, root := range roots {
		counts[filepath.Base(root)]++
	}
	labels := make(map[string]string, len(roots))
	for _, root := range roots {
		label := filepath.Base(root)
		if counts[label] > 1 {
			label = root
		}
		labels[root] = label
	}
	return labels
}

func statusRow(s status.RepoStatus, showRemote bool) []string {
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
	return row
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
	return writeTableWidths(w, rows, widths)
}

func writeTableWidths(w io.Writer, rows [][]string, widths []int) error {
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
