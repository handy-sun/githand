package display

import (
	"bytes"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
	"unicode"

	"github.com/handy-sun/githand/internal/config"
	"github.com/handy-sun/githand/internal/i18n"
	"github.com/handy-sun/githand/internal/status"
)

func TestStatusTableAlignsChineseColumnsByDisplayWidth(t *testing.T) {
	i18n.SetLocale("zh")
	t.Cleanup(func() { i18n.SetLocale("en") })

	output := captureStdout(t, func() {
		err := Status([]status.RepoStatus{
			{
				Repo:       config.Repo{Name: "githand"},
				Branch:     "main",
				Dirty:      false,
				Ahead:      3,
				Behind:     0,
				StashCount: 0,
			},
		}, false)
		if err != nil {
			t.Fatal(err)
		}
	})

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected header and one row, got %d lines:\n%s", len(lines), output)
	}

	headerStarts := columnStarts(lines[0], []string{"仓库", "分支", "状态", "领先", "落后", "暂存"})
	rowStarts := columnStarts(lines[1], []string{"githand", "main", "干净", "3", "0", "0"})

	if !reflect.DeepEqual(rowStarts, headerStarts) {
		t.Fatalf("column starts differ by display width\nheader: %v %q\nrow:    %v %q", headerStarts, lines[0], rowStarts, lines[1])
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer

	fn()

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = original

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func columnStarts(line string, cells []string) []int {
	starts := make([]int, 0, len(cells))
	searchFrom := 0
	for _, cell := range cells {
		idx := strings.Index(line[searchFrom:], cell)
		if idx < 0 {
			return starts
		}
		start := searchFrom + idx
		starts = append(starts, displayWidthForTest(line[:start]))
		searchFrom = start + len(cell)
	}
	return starts
}

func displayWidthForTest(s string) int {
	width := 0
	for _, r := range s {
		if unicode.In(r, unicode.Han, unicode.Hangul, unicode.Hiragana, unicode.Katakana) {
			width += 2
			continue
		}
		width++
	}
	return width
}
