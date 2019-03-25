/////////////////////////////////////////////////////////////////////////////////
//
// rungroup.go
//
// Written by Lorenz Breidenbach lob@open.ch, December 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package capture

import "sync"

// RunGroup wraps the common waitgroup setup for go routines that need to finish before
// continuing with instruction execution.
type RunGroup struct {
	wg sync.WaitGroup
}

// Run executes any function inside a go routine and waits for it
func (rg *RunGroup) Run(f func()) {
	rg.wg.Add(1)
	go func() {
		defer rg.wg.Done()
		f()
	}()
}

// Wait wraps the sync.Wait method
func (rg *RunGroup) Wait() {
	rg.wg.Wait()
}
