package goDB

import "os"

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

	return ifaces, nil
}
