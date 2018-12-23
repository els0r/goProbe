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

type RunGroup struct {
	wg sync.WaitGroup
}

func (rg *RunGroup) Run(f func()) {
	rg.wg.Add(1)
	go func() {
		defer rg.wg.Done()
		f()
	}()
}

func (rg *RunGroup) Wait() {
	rg.wg.Wait()
}
