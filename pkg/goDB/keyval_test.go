package goDB

import (
	"encoding/binary"
	"testing"

	jsoniter "github.com/json-iterator/go"
)

func TestJSONMarshalAggFlowMap(t *testing.T) {

	var ip [16]byte
	m := AggFlowMap{
		string(NewKey(ip[:], ip[:], []byte{0, 0}, 0x11)): &Val{1, 1, 0, 0, 0, ""},
		string(NewKey(ip[:], ip[:], []byte{0, 0}, 0x06)): &Val{2, 2, 0, 0, 0, ""},
	}

	b, err := jsoniter.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal aggregated flow map: %s", err)
	}
	_ = b
}

func BenchmarkNativeMapAccess(b *testing.B) {

	testMap := make(AggFlowMap)
	for i := 0; i < 1000000; i++ {
		var ip [16]byte
		binary.BigEndian.PutUint64(ip[:], uint64(i))

		testMap[string(NewKey(ip[:], ip[:], []byte{0, 0}, 0))] = &Val{
			NBytesRcvd: uint64(i),
			NBytesSent: uint64(i),
			NPktsRcvd:  uint64(i),
			NPktsSent:  uint64(i),
		}
	}

	b.ReportAllocs()
	b.ResetTimer()

	checkKey := Key{}

	for i := 0; i < b.N; i++ {
		_ = testMap[string(checkKey)]
	}
}

type flatByteArrayKey [35]byte

func BenchmarkByteArrayMapAccess(b *testing.B) {

	testMap := make(map[flatByteArrayKey]*Val)
	for i := 0; i < 1000000; i++ {
		var key flatByteArrayKey
		binary.BigEndian.PutUint64(key[:8], uint64(i))
		binary.BigEndian.PutUint64(key[8:16], uint64(i))

		testMap[key] = &Val{
			NBytesRcvd: uint64(i),
			NBytesSent: uint64(i),
			NPktsRcvd:  uint64(i),
			NPktsSent:  uint64(i),
		}
	}

	b.ReportAllocs()
	b.ResetTimer()

	checkKey := flatByteArrayKey{}

	for i := 0; i < b.N; i++ {
		_ = testMap[checkKey]
	}
}

func BenchmarkStringedByteArrayMapAccess(b *testing.B) {

	testMap := make(map[string]*Val)
	for i := 0; i < 1000000; i++ {
		var key flatByteArrayKey
		binary.BigEndian.PutUint64(key[:8], uint64(i))
		binary.BigEndian.PutUint64(key[8:16], uint64(i))

		testMap[string(key[:])] = &Val{
			NBytesRcvd: uint64(i),
			NBytesSent: uint64(i),
			NPktsRcvd:  uint64(i),
			NPktsSent:  uint64(i),
		}
	}

	b.ReportAllocs()
	b.ResetTimer()

	checkKey := flatByteArrayKey{}

	for i := 0; i < b.N; i++ {
		_ = testMap[string(checkKey[:])]
	}
}

func BenchmarkStringedByteSliceMapAccess(b *testing.B) {

	testMap := make(map[string]*Val)
	for i := 0; i < 1000000; i++ {
		var key = make([]byte, 35)
		binary.BigEndian.PutUint64(key[:8], uint64(i))
		binary.BigEndian.PutUint64(key[8:16], uint64(i))

		testMap[string(key)] = &Val{
			NBytesRcvd: uint64(i),
			NBytesSent: uint64(i),
			NPktsRcvd:  uint64(i),
			NPktsSent:  uint64(i),
		}
	}

	b.ReportAllocs()
	b.ResetTimer()

	checkKey := make([]byte, 35)

	for i := 0; i < b.N; i++ {
		_ = testMap[string(checkKey)]
	}
}
