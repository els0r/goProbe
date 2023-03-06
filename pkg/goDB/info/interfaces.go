package info

import (
	"os"
	"sort"
)

// GetInterfaces returns a list of interfaces covered by this goDB
// TODO: This needs some extension to also allow for access to metadata
// beyond the names
func GetInterfaces(dbPath string) ([]string, error) {
	dirents, err := os.ReadDir(dbPath)
	if err != nil {
		return nil, err
	}

	var ifaces []string
	for _, dirent := range dirents {
		if dirent.IsDir() {
			ifaces = append(ifaces, dirent.Name())
		}
	}
	sort.SliceStable(ifaces, func(i, j int) bool {
		return ifaces[i] < ifaces[j]
	})

	return ifaces, nil
}
