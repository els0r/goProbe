package shellformat

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// ShellFormat denotes a simple formatting modifier for shell output
type ShellFormat = string

var isNoColorTerm = os.Getenv("TERM") == "dumb" || !isTerminal(os.Stdout.Fd())

// Some standard modifiers
var (
	Reset ShellFormat = "\033[0m"
	Bold  ShellFormat = "\033[1m"

	Black  ShellFormat = "\033[30m"
	Red    ShellFormat = "\033[31m"
	Green  ShellFormat = "\033[32m"
	Yellow ShellFormat = "\033[33m"
	Blue   ShellFormat = "\033[34m"
	Purple ShellFormat = "\033[35m"
	Cyan   ShellFormat = "\033[36m"
	Gray   ShellFormat = "\033[37m"
	White  ShellFormat = "\033[97m"
)

// FormatShell modifies the provided string using the list of modifiers
// and resets the output formatting to default at the end
func FormatShell(input interface{}, formats ...ShellFormat) string {

	// If this terminal / shell cannot display color / formatting modifiers, skip
	if isNoColorTerm {
		return fmt.Sprint(input)
	}

	var prefix string
	for _, format := range formats {
		prefix += format
	}

	return prefix + fmt.Sprint(input) + Reset
}

func isTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetTermios(int(fd), unix.TCGETS)
	return err == nil
}
