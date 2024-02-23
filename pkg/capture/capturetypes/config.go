package capturetypes

// IfaceChange denotes the result from a config update / reload of an interface
type IfaceChange struct {
	Name    string // Name: the name of the interface. Example: "eth0"
	Success bool   // Success: the config update / reload operation(s) succeeded. Example: true
}

// IfaceChanges denotes a list of IfaceChange instances
type IfaceChanges []IfaceChange

// Names return a simple string slice containing all interface names
func (c IfaceChanges) Names() []string {
	names := make([]string, len(c))
	for i := 0; i < len(c); i++ {
		names[i] = c[i].Name
	}
	return names
}

// Results return both successful and failed results in a slice, respectively
func (c IfaceChanges) Results() (ok []string, failed []string) {
	for _, change := range c {
		if change.Success {
			ok = append(ok, change.Name)
		} else {
			failed = append(failed, change.Name)
		}
	}

	return
}

// Len returns the length (read: number) of interface changes (implementation of sorting interface)
func (c IfaceChanges) Len() int {
	return len(c)
}

// Less returns if a named change is to be ordered before a second one (implementation of sorting interface)
func (c IfaceChanges) Less(i, j int) bool {
	return c[i].Name < c[j].Name
}

// Swap swaps two interface changes in the list (implementation of sorting interface)
func (c IfaceChanges) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

// FromIfaceNames generates a list of IfaceChange instances from a list of interface names
func FromIfaceNames(names []string) IfaceChanges {
	res := make(IfaceChanges, len(names))
	for i := 0; i < len(names); i++ {
		res[i].Name = names[i]
	}
	return res
}
