/////////////////////////////////////////////////////////////////////////////////
//
// readint_test.go
//
// Written by Lorenz Breidenbach lob@open.ch, November 2015
// Copyright (c) 2015 Open Systems AG, Switzerland
// All Rights Reserved.
//
/////////////////////////////////////////////////////////////////////////////////

package bigendian

import (
	"math/rand"
	"testing"
)

func callReadUint64At(b []byte, idx int) (result uint64, panicked bool) {
	defer func() {
		if e := recover(); e != nil {
			panicked = true
		}
	}()
	result = ReadUint64At(b, idx)
	return
}

func callUnsafeReadUint64At(b []byte, idx int) (result uint64, panicked bool) {
	defer func() {
		if e := recover(); e != nil {
			panicked = true
		}
	}()
	result = UnsafeReadUint64At(b, idx)
	return
}

func callReadUint64AtRef(b []byte, idx int) (result uint64, panicked bool) {
	defer func() {
		if e := recover(); e != nil {
			panicked = true
		}
	}()
	result = readUint64AtRef(b, idx)
	return
}

func callReadInt64At(b []byte, idx int) (result int64, panicked bool) {
	defer func() {
		if e := recover(); e != nil {
			panicked = true
		}
	}()
	result = ReadInt64At(b, idx)
	return
}

func callUnsafeReadInt64At(b []byte, idx int) (result int64, panicked bool) {
	defer func() {
		if e := recover(); e != nil {
			panicked = true
		}
	}()
	result = UnsafeReadInt64At(b, idx)
	return
}

func callReadInt64AtRef(b []byte, idx int) (result int64, panicked bool) {
	defer func() {
		if e := recover(); e != nil {
			panicked = true
		}
	}()
	result = readInt64AtRef(b, idx)
	return
}

var tests = [][]byte{
	{},
	{1},
	{1, 2},
	{1, 2, 3},
	{1, 2, 3, 4},
	{1, 2, 3, 4, 5},
	{1, 2, 3, 4, 5, 6},
	{1, 2, 3, 4, 5, 6, 7},
	{1, 2, 3, 4, 5, 6, 7, 8},
	{0xFD, 0xCB, 0xA9, 0x87, 0x65, 0x43, 0x21, 0x99},
	{1, 2, 3, 4, 5, 6, 7, 8, 9},
	{0xFD, 0xCB, 0xA9, 0x87, 0x65, 0x43, 0x21, 0x99, 0x72},
}

func randBytes(n int) []byte {
	var b []byte
	for i := 0; i < n; i++ {
		b = append(b, byte(rand.Int()))
	}
	return b
}

func TestReadUint64AtZero(t *testing.T) {
	for _, test := range tests {
		expectedResult, expectedPanicked := callReadUint64AtRef(test, 0)
		actualResult, actualPanicked := callReadUint64At(test, 0)
		if expectedPanicked != actualPanicked || expectedResult != actualResult {
			t.Fatalf("results for %v dont match.", test)
		}
	}
}

func TestReadUint64AtRandom(t *testing.T) {
	b := randBytes(10000)

	for shift := 0; shift < 8; shift++ {
		for i := 0; i <= cap(b)*2; i++ {
			expectedResult, expectedPanicked := callReadUint64AtRef(b[shift:], i)
			actualResult, actualPanicked := callReadUint64At(b[shift:], i)
			if expectedPanicked != actualPanicked || expectedResult != actualResult {
				t.Fatalf("index: %v shift: %v expected: %v, %v got: %v, %v",
					i, shift, expectedResult, expectedPanicked, actualResult, actualPanicked)
			}
		}
	}
}

func TestUnsafeReadUint64AtRandom(t *testing.T) {
	b := randBytes(10000)

	for shift := 0; shift < 8; shift++ {
		for i := 0; i <= cap(b)*2; i++ {
			expectedResult, expectedPanicked := callReadUint64AtRef(b[shift:], i)
			// The unsafe call never panics. So if the reference panicked,
			// there is no point in doing a comparison.
			if expectedPanicked {
				continue
			}
			actualResult, actualPanicked := callUnsafeReadUint64At(b[shift:], i)
			if expectedPanicked != actualPanicked || expectedResult != actualResult {
				t.Fatalf("index: %v shift: %v expected: %v, %v got: %v, %v",
					i, shift, expectedResult, expectedPanicked, actualResult, actualPanicked)
			}
		}
	}
}

func TestReadInt64AtZero(t *testing.T) {
	for _, test := range tests {
		expectedResult, expectedPanicked := callReadInt64AtRef(test, 0)
		actualResult, actualPanicked := callReadInt64At(test, 0)
		if expectedPanicked != actualPanicked || expectedResult != actualResult {
			t.Fatalf("results for %v dont match.", test)
		}
	}
}

func TestReadInt64AtRandom(t *testing.T) {
	b := randBytes(10000)

	for shift := 0; shift < 8; shift++ {
		for i := 0; i <= cap(b)*2; i++ {
			expectedResult, expectedPanicked := callReadInt64AtRef(b[shift:], i)
			actualResult, actualPanicked := callReadInt64At(b[shift:], i)
			if expectedPanicked != actualPanicked || expectedResult != actualResult {
				t.Fatalf("index: %v shift: %v expected: %v, %v got: %v, %v",
					i, shift, expectedResult, expectedPanicked, actualResult, actualPanicked)
			}
		}
	}
}

func TestUnsafeReadInt64AtRandom(t *testing.T) {
	b := randBytes(10000)

	for shift := 0; shift < 8; shift++ {
		for i := 0; i <= cap(b)*2; i++ {
			expectedResult, expectedPanicked := callReadInt64AtRef(b[shift:], i)
			// The unsafe call never panics. So if the reference panicked,
			// there is no point in doing a comparison.
			if expectedPanicked {
				continue
			}
			actualResult, actualPanicked := callUnsafeReadInt64At(b[shift:], i)
			if expectedPanicked != actualPanicked || expectedResult != actualResult {
				t.Fatalf("index: %v shift: %v expected: %v, %v got: %v, %v",
					i, shift, expectedResult, expectedPanicked, actualResult, actualPanicked)
			}
		}
	}
}

const QWORDS = 1024 * 1024 * 4

func BenchmarkUnsafeReadUint64At(b *testing.B) {
	b.SetBytes(4 * 8 * QWORDS)
	var n = QWORDS * b.N
	var data [4][]byte
	for i := 0; i < 4; i++ {
		for j := 0; j < 8*n; j++ {
			data[i] = append(data[i], byte(i*j))
		}
	}
	b.ResetTimer()
	var sum [4]uint64
	for j := 0; j < n; j++ {
		for i := 0; i < 4; i++ {
			sum[i] += UnsafeReadUint64At(data[i], j)
		}
	}
}

func BenchmarkReadUint64At(b *testing.B) {
	b.SetBytes(4 * 8 * QWORDS)
	var n = QWORDS * b.N
	var data [4][]byte
	for i := 0; i < 4; i++ {
		for j := 0; j < 8*n; j++ {
			data[i] = append(data[i], byte(i*j))
		}
	}
	b.ResetTimer()
	var sum [4]uint64
	for j := 0; j < n; j++ {
		for i := 0; i < 4; i++ {
			sum[i] += ReadUint64At(data[i], j)
		}
	}
}

func BenchmarkReadUint64AtRef(b *testing.B) {
	b.SetBytes(4 * 8 * QWORDS)
	var n = QWORDS * b.N
	var data [4][]byte
	for i := 0; i < 4; i++ {
		for j := 0; j < 8*n; j++ {
			data[i] = append(data[i], byte(i*j))
		}
	}
	b.ResetTimer()
	var sum [4]uint64
	for j := 0; j < n; j++ {
		for i := 0; i < 4; i++ {
			sum[i] += readUint64AtRef(data[i], j)
		}
	}
}
