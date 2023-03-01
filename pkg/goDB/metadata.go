/////////////////////////////////////////////////////////////////////////////////
//
// metadata.go
//
// Written by Lorenz Breidenbach lob@open.ch, January 2016
// Copyright (c) 2016 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package goDB

// CaptureMetadata represents metadata for one database block.
// TODO: This will be replaced by global Capture Stats when merging #47
type CaptureMetadata struct {
	PacketsDropped int
}
