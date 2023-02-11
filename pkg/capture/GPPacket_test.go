/////////////////////////////////////////////////////////////////////////////////
//
// GPPacket.go
//
// Testing file for GPPacket allocation and handling
//
// Written by Fabian Kohn fko@open.ch, June 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package capture

import "testing"

func BenchmarkAllocateIn(b *testing.B) {
	var g *GPPacket
	for i := 0; i < b.N; i++ {
		g = &GPPacket{
			numBytes:   100,
			dirInbound: true,
		}
	}

	_ = g
}

func BenchmarkAllocateOut(b *testing.B) {
	var g *GPPacket
	for i := 0; i < b.N; i++ {
		g = &GPPacket{
			numBytes:   100,
			dirInbound: false,
		}
	}

	_ = g
}
