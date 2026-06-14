package display

import (
	"fmt"
	"os"
)

// ANSI color codes.
const (
	greenCode = "\033[32m"
	resetCode = "\033[0m"
)

// noColor is true when output should omit ANSI escape sequences.
// Set via NO_COLOR env var or when stdout is not a terminal.
var noColor = os.Getenv("NO_COLOR") != ""

// Green wraps s in green ANSI codes (unless NO_COLOR is set).
func Green(s string) string {
	if noColor {
		return s
	}
	return greenCode + s + resetCode
}

// Greenf formats and wraps in green ANSI codes.
func Greenf(format string, a ...any) string {
	return Green(fmt.Sprintf(format, a...))
}
