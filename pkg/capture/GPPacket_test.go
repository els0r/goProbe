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
	for i := 0; i < b.N; i++ {
		NewGPPacket([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, [2]byte{1, 2}, [2]byte{1, 2}, [4]byte{1, 2, 3, 4}, 4, 17, 128, 0, true)
	}
}

func BenchmarkAllocateOut(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewGPPacket([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, [2]byte{1, 2}, [2]byte{1, 2}, [4]byte{1, 2, 3, 4}, 4, 17, 128, 0, false)
	}
}
