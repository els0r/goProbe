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
	if d/time.Hour != 0 {
		return fmt.Sprintf("%dh%2dm", d/time.Hour, d%time.Hour/time.Minute)
	}
	if d/time.Minute != 0 {
		return fmt.Sprintf("%dm%2ds", d/time.Minute, d%time.Minute/time.Second)
	}
	if d/time.Second != 0 {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%dms", d/time.Millisecond)
}
