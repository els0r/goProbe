package formatting

import (
	"fmt"
	"time"
)

// Countable is a uint64 that can be printed in a human readable format
type Countable uint64

// String prints the Countable in a human readable format
func (c Countable) String() string {
	return Count(uint64(c))
}

// Sizeable is a number of bytes that can be printed in a human readable format
type Sizeable uint64

// String prints the Sizeable in a human readable format
func (s Sizeable) String() string {
	return Size(uint64(s))
}

// Durationable is a time.Duration that can be printed in a human readable format and
// will prepend 'd' for 'days' in case the duration is above 24 hours
type Durationable time.Duration

func (d Durationable) String() string {
	return Duration(time.Duration(d))
}

// Count takes a number and prints it in a human readable format,
// e.g. 1000 -> 1k, 1000000 -> 1M, 1000000000 -> 1G
func Count(val uint64) string {
	count := 0
	var valF = float64(val)

	units := []string{" ", "k", "M", "G", "T", "P", "E", "Z", "Y"}

	for val >= 1000 {
		val /= 1000
		valF /= 1000.0
		count++
	}
	if valF == 0 {
		return fmt.Sprintf("%.2f %s", valF, units[count])
	}

	return fmt.Sprintf("%.2f %s", valF, units[count])
}

// Size prints out size in a human-readable format (e.g. 10 MB)
func Size(size uint64) string {
	count := 0
	var sizeF = float64(size)

	units := []string{" B", "kB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"}

	for size > 1024 {
		size /= 1024
		sizeF /= 1024.0
		count++
	}
	if sizeF == 0 {
		return fmt.Sprintf("%.2f %s", sizeF, units[count])
	}

	return fmt.Sprintf("%.2f %s", sizeF, units[count])
}

// Duration prints out d in a human-readable duration format
func Duration(d time.Duration) string {
	// enhance the classic duration Stringer to print out days
	days := d / (24 * time.Hour)
	if days != 0 {
		d = d - (days * 24 * time.Hour)
		return fmt.Sprintf("%dd%s", days, d.Round(time.Millisecond))
	}
	return d.Round(time.Millisecond).String()
}
