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
			sip:        [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			dip:        [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			sport:      [2]byte{1, 2},
			dport:      [2]byte{1, 2},
			protocol:   17,
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
			sip:        [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			dip:        [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			sport:      [2]byte{1, 2},
			dport:      [2]byte{1, 2},
			protocol:   17,
			numBytes:   100,
			dirInbound: false,
		}
	}

	_ = g
}
