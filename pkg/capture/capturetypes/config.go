package capturetypes

// IfaceChange denotes the result from an interface / config update of an interface
type IfaceChange struct {
	Name    string
	Success bool
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

// FromIfaceNames generates a list of IfaceChange instances from a list of interface names
func FromIfaceNames(names []string) IfaceChanges {
	res := make(IfaceChanges, len(names))
	for i := 0; i < len(names); i++ {
		res[i].Name = names[i]
	}
	return res
}
