package capture

import "sync"

// Captures denotes a named set of Capture instances, wrapping a map and the
// required synchronization of all its actions
type captures struct {
	Map map[string]*Capture
	sync.RWMutex
}

// newCaptures instantiates a new, empty set of Captures
func newCaptures() *captures {
	return &captures{
		Map:     make(map[string]*Capture),
		RWMutex: sync.RWMutex{},
	}
}

// Ifaces return the list of names of all interfaces in the set
func (c *captures) Ifaces(ifaces ...string) []string {
	if len(ifaces) == 0 {
		c.RLock()
		ifaces = make([]string, 0, len(c.Map))
		for iface := range c.Map {
			ifaces = append(ifaces, iface)
		}
		c.RUnlock()
	}

	return ifaces
}

// Get safely returns a Capture by name (and an indicator if it exists)
func (c *captures) Get(iface string) (capture *Capture, exists bool) {
	c.RLock()
	capture, exists = c.GetNoLock(iface)
	c.RUnlock()
	return
}

// GetNoLock returns a Capture by name (and an indicator if it exists) without locking
func (c *captures) GetNoLock(iface string) (capture *Capture, exists bool) {
	capture, exists = c.Map[iface]
	return
}

// Set safely adds / overwrites a Capture by name
func (c *captures) Set(iface string, capture *Capture) {
	c.Lock()
	c.SetNoLock(iface, capture)
	c.Unlock()
}

// SetNoLock adds / overwrites a Capture by name without locking
func (c *captures) SetNoLock(iface string, capture *Capture) {
	c.Map[iface] = capture
}

// Delete safely removes a Capture from the set by name
func (c *captures) Delete(iface string) {
	c.Lock()
	c.DeleteNoLock(iface)
	c.Unlock()
}

// Delete removes a Capture from the set by name without locking
func (c *captures) DeleteNoLock(iface string) {
	delete(c.Map, iface)
}
