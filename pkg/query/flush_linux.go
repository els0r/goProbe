//+build linux

package query

var syncCmd = []string{"sync", "&&", "echo", "3", ">", "/proc/sys/vm/drop_caches"}
