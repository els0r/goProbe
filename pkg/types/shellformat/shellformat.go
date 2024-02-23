package shellformat

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// EscapeSeq denotes a simple formatting modifier / escape sequence for shell output
type EscapeSeq = string

var isNoColorTerm = os.Getenv("TERM") == "dumb" || !isTerminal(os.Stdout.Fd())

// Some standard modifiers
var (
	EscapeSeqReset  EscapeSeq = "\033[0m"
	EscapeSeqBold   EscapeSeq = "\033[1m"
	EscapeSeqBlack  EscapeSeq = "\033[30m"
	EscapeSeqRed    EscapeSeq = "\033[31m"
	EscapeSeqGreen  EscapeSeq = "\033[32m"
	EscapeSeqYellow EscapeSeq = "\033[33m"
	EscapeSeqBlue   EscapeSeq = "\033[34m"
	EscapeSeqPurple EscapeSeq = "\033[35m"
	EscapeSeqCyan   EscapeSeq = "\033[36m"
	EscapeSeqGray   EscapeSeq = "\033[37m"
	EscapeSeqWhite  EscapeSeq = "\033[97m"
)

// Format denotes an abstract format for shell output
type Format uint64

// Formatting codes suitable for logical combination via '|'
const (
	Bold   Format = 1 << 1
	Black  Format = 1 << 2
	Red    Format = 1 << 3
	Green  Format = 1 << 4
	Yellow Format = 1 << 5
	Blue   Format = 1 << 6
	Purple Format = 1 << 7
	Cyan   Format = 1 << 8
	Gray   Format = 1 << 9
	White  Format = 1 << 10

	maxFormat = White
)

var allFormats = []EscapeSeq{
	"",
	EscapeSeqBold,
	EscapeSeqBlack,
	EscapeSeqRed,
	EscapeSeqGreen,
	EscapeSeqYellow,
	EscapeSeqBlue,
	EscapeSeqPurple,
	EscapeSeqCyan,
	EscapeSeqGray,
	EscapeSeqWhite,
}

// Fmt modifies the provided string using the list of modifiers
// and resets the output formatting to default at the end
func Fmt(format Format, input string, a ...any) string {

	if isNoColorTerm {
		return fmt.Sprintf(input, a...)
	}

	seq := format.genEscapeSeq()
	if seq == "" {
		return fmt.Sprintf(input, a...)
	}

	return fmt.Sprintf(seq+input+EscapeSeqReset, a...)
}

func (f Format) genEscapeSeq() (seq string) {
	for i := 1; i <= int(maxFormat); i++ {
		if f&(1<<i) != 0 {
			seq += allFormats[i]
		}
	}
	return
}

func isTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetTermios(int(fd), unix.TCGETS)
	return err == nil
}
