//go:build linux
// +build linux

package engine

var syncCmd = []string{"sync", "&&", "echo", "3", ">", "/proc/sys/vm/drop_caches"}
