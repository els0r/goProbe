package hashmap

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/els0r/goProbe/pkg/types"
	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/require"
)

func TestSeedRandomness(t *testing.T) {
	var lastSeed uint64
	for i := 0; i < 1000; i++ {
		testMap := New()
		require.NotEqual(t, 0, testMap.seed)
		require.NotEqual(t, lastSeed, testMap.seed)
		lastSeed = testMap.seed
	}
}

func TestSimpleHashMapOperations(t *testing.T) {

	testMap := New()
	testMap.Set([]byte("a"), types.Counters{BytesRcvd: 0, BytesSent: uint64([]byte("a")[0]), PacketsRcvd: 0, PacketsSent: 0})
	testMap.Set([]byte("b"), types.Counters{BytesRcvd: 0, BytesSent: uint64([]byte("b")[0]), PacketsRcvd: 0, PacketsSent: 0})
	testMap.Set([]byte("c"), types.Counters{BytesRcvd: 0, BytesSent: uint64([]byte("c")[0]), PacketsRcvd: 0, PacketsSent: 0})

	val, exists := testMap.Get([]byte("a"))
	require.True(t, exists)
	require.Equal(t, types.Counters{BytesRcvd: 0, BytesSent: uint64([]byte("a")[0]), PacketsRcvd: 0, PacketsSent: 0}, val)
	val, exists = testMap.Get([]byte("b"))
	require.True(t, exists)
	require.Equal(t, types.Counters{BytesRcvd: 0, BytesSent: uint64([]byte("b")[0]), PacketsRcvd: 0, PacketsSent: 0}, val)
	val, exists = testMap.Get([]byte("c"))
	require.True(t, exists)
	require.Equal(t, types.Counters{BytesRcvd: 0, BytesSent: uint64([]byte("c")[0]), PacketsRcvd: 0, PacketsSent: 0}, val)

	val, exists = testMap.Get([]byte("d"))
	require.False(t, exists)
	require.Equal(t, types.Counters{BytesRcvd: 0, BytesSent: 0, PacketsRcvd: 0, PacketsSent: 0}, val)

	var count int
	for i := testMap.Iter(); i.Next(); {
		count++
		require.Equal(t, uint64(i.Key()[0]), i.Val().BytesSent)
	}
	require.Equal(t, count, testMap.Len())
}

func TestHashMapSetOrUpdate(t *testing.T) {

	testMap := New()
	testMap.Set([]byte("a"), types.Counters{BytesRcvd: 0, BytesSent: uint64([]byte("a")[0]), PacketsRcvd: 0, PacketsSent: 0})
	testMap.Set([]byte("b"), types.Counters{BytesRcvd: 0, BytesSent: uint64([]byte("b")[0]), PacketsRcvd: 0, PacketsSent: 0})
	testMap.Set([]byte("c"), types.Counters{BytesRcvd: 0, BytesSent: uint64([]byte("c")[0]), PacketsRcvd: 0, PacketsSent: 0})

	val, exists := testMap.Get([]byte("a"))
	require.True(t, exists)
	require.Equal(t, types.Counters{BytesRcvd: 0, BytesSent: uint64([]byte("a")[0]), PacketsRcvd: 0, PacketsSent: 0}, val)

	testMap.SetOrUpdate([]byte("a"), 0, 2, 0, 1)
	testMap.SetOrUpdate([]byte("b"), 0, 10000, 10, 1)

	val, exists = testMap.Get([]byte("a"))
	require.True(t, exists)
	require.Equal(t, types.Counters{BytesRcvd: 0, BytesSent: uint64([]byte("a")[0]) + 2, PacketsRcvd: 0, PacketsSent: 1}, val)
	val, exists = testMap.Get([]byte("b"))
	require.True(t, exists)
	require.Equal(t, types.Counters{BytesRcvd: 0, BytesSent: uint64([]byte("b")[0]) + 10000, PacketsRcvd: 10, PacketsSent: 1}, val)
	val, exists = testMap.Get([]byte("c"))
	require.True(t, exists)
	require.Equal(t, types.Counters{BytesRcvd: 0, BytesSent: uint64([]byte("c")[0]), PacketsRcvd: 0, PacketsSent: 0}, val)

	var count int
	for i := testMap.Iter(); i.Next(); {
		count++
	}
	require.Equal(t, count, testMap.Len())
}

func TestHashMapIteratorConsistency(t *testing.T) {
	testMap := New()
	for i := 0; i < 1000; i++ {
		temp := make([]byte, 8)
		binary.BigEndian.PutUint64(temp, uint64(i))
		testMap.Set(temp, types.Counters{BytesRcvd: uint64(i), BytesSent: 0, PacketsRcvd: 0, PacketsSent: 0})

		var count int
		for it := testMap.Iter(); it.Next(); {
			count++
		}
		require.Equal(t, i+1, testMap.Len())
		require.Equal(t, testMap.Len(), count)
	}
}

func TestAggFlowMapFlatten(t *testing.T) {
	testMap := NewAggFlowMap()
	for i := 0; i < 1000; i++ {
		temp := make([]byte, 8)
		binary.BigEndian.PutUint64(temp, uint64(i))
		testMap.PrimaryMap.Set(temp, types.Counters{BytesRcvd: uint64(i), BytesSent: 0, PacketsRcvd: 0, PacketsSent: 0})
		testMap.SecondaryMap.Set(temp, types.Counters{BytesRcvd: uint64(i), BytesSent: 0, PacketsRcvd: 0, PacketsSent: 0})
		var count int
		for it := testMap.Iter(); it.Next(); {
			count++
		}
		require.Equal(t, 2*(i+1), testMap.Len())
		require.Equal(t, testMap.Len(), count)

		primaryList, secondaryList := testMap.Flatten()
		require.Equal(t, i+1, len(primaryList))
		require.Equal(t, i+1, len(secondaryList))
	}
}

func TestLinearHashMapOperations(t *testing.T) {

	testMap := New()
	for i := 0; i < 100000; i++ {
		temp := make([]byte, 8)
		binary.BigEndian.PutUint64(temp, uint64(i))
		testMap.Set(temp, types.Counters{BytesRcvd: uint64(i), BytesSent: 0, PacketsRcvd: 0, PacketsSent: 0})
	}

	require.Equal(t, 100000, testMap.Len())

	for i := 0; i < 100000; i++ {
		temp := make([]byte, 8)
		binary.BigEndian.PutUint64(temp, uint64(i))
		val, exists := testMap.Get(temp)
		require.True(t, exists)
		require.Equal(t, types.Counters{BytesRcvd: uint64(i), BytesSent: 0, PacketsRcvd: 0, PacketsSent: 0}, val)
	}
}

func TestMerge(t *testing.T) {

	testMap, testMap2 := New(), New(100000)
	for i := 0; i < 110000; i++ {
		temp := make([]byte, 8)
		binary.BigEndian.PutUint64(temp, uint64(i))
		if i < 100000 {
			testMap.Set(temp, types.Counters{BytesRcvd: uint64(i), BytesSent: 0, PacketsRcvd: 0, PacketsSent: 0})
		}
		if i >= 50000 {
			testMap2.Set(temp, types.Counters{BytesRcvd: uint64(i), BytesSent: 0, PacketsRcvd: 0, PacketsSent: 0})
		}
	}

	require.Equal(t, 100000, testMap.Len())
	require.Equal(t, 60000, testMap2.Len())

	var (
		mergeMap = New()
		totals   Val
	)

	mergeMap.Merge(testMap, &totals)

	require.Equal(t, testMap.Len(), mergeMap.Len())
	require.Equal(t, 100000, testMap.Len())
	require.Equal(t, 60000, testMap2.Len())

	mergeMap.Merge(testMap2, &totals)

	require.Equal(t, 110000, mergeMap.Len())
	require.Equal(t, 100000, testMap.Len())
	require.Equal(t, 60000, testMap2.Len())
}

func TestJSONMarshalAggFlowMap(t *testing.T) {

	var ip [16]byte
	m := New()
	m.Set(types.NewKey(ip[:], ip[:], []byte{0, 0}, 0x11), types.Counters{BytesRcvd: 1, BytesSent: 1, PacketsRcvd: 0, PacketsSent: 0})
	m.Set(types.NewKey(ip[:], ip[:], []byte{0, 0}, 0x06), types.Counters{BytesRcvd: 2, BytesSent: 2, PacketsRcvd: 0, PacketsSent: 0})

	b, err := jsoniter.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal aggregated flow map: %s", err)
	}
	_ = b
}

var globalV types.Counters
var globalExists bool

func BenchmarkHashMapAccesses(b *testing.B) {
	for _, nElem := range []int{8, 100000} {
		b.Run(fmt.Sprintf("%d elem", nElem), func(b *testing.B) {
			testMap := New()
			for i := 0; i < nElem; i++ {
				temp := make([]byte, 8)
				binary.BigEndian.PutUint64(temp, uint64(i))
				testMap.Set(temp, types.Counters{BytesRcvd: uint64(i), BytesSent: 0, PacketsRcvd: 0, PacketsSent: 0})
			}

			testKey := make([]byte, 8)
			binary.BigEndian.PutUint64(testKey, 42)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				v, exists := testMap.Get(testKey)
				globalV = v
				globalExists = exists
			}
		})
	}
}

func BenchmarkHashMapMisses(b *testing.B) {
	for _, nElem := range []int{8, 100000} {
		b.Run(fmt.Sprintf("%d elem", nElem), func(b *testing.B) {
			testMap := New()
			for i := 0; i < nElem; i++ {
				temp := make([]byte, 8)
				binary.BigEndian.PutUint64(temp, uint64(i))
				testMap.Set(temp, types.Counters{BytesRcvd: uint64(i), BytesSent: 0, PacketsRcvd: 0, PacketsSent: 0})
			}

			testKey := make([]byte, 8)
			binary.BigEndian.PutUint64(testKey, 1000000)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				v, exists := testMap.Get(testKey)
				globalV = v
				globalExists = exists
			}
		})
	}
}

func BenchmarkHashMapIterator(b *testing.B) {

	testMap := New()
	for i := 0; i < 100000; i++ {
		temp := make([]byte, 8)
		binary.BigEndian.PutUint64(temp, uint64(i))
		testMap.Set(temp, types.Counters{BytesRcvd: uint64(i), BytesSent: 0, PacketsRcvd: 0, PacketsSent: 0})
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for it := testMap.Iter(); it.Next(); {
			_ = it.Key()
			_ = it.Val()
		}
	}
}

func BenchmarkNativeMapAccesses(b *testing.B) {
	for _, nElem := range []int{8, 100000} {
		b.Run(fmt.Sprintf("%d elem", nElem), func(b *testing.B) {
			testMap := make(map[string]types.Counters)
			for i := 0; i < nElem; i++ {
				temp := make([]byte, 8)
				binary.BigEndian.PutUint64(temp, uint64(i))
				testMap[string(temp)] = types.Counters{BytesRcvd: uint64(i), BytesSent: 0, PacketsRcvd: 0, PacketsSent: 0}
			}

			testKey := make([]byte, 8)
			binary.BigEndian.PutUint64(testKey, 42)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				v, exists := testMap[string(testKey)]
				globalV = v
				globalExists = exists
			}
		})
	}
}

func BenchmarkNativeMapMisses(b *testing.B) {
	for _, nElem := range []int{8, 100000} {
		b.Run(fmt.Sprintf("%d elem", nElem), func(b *testing.B) {
			testMap := make(map[string]types.Counters)
			for i := 0; i < nElem; i++ {
				temp := make([]byte, 8)
				binary.BigEndian.PutUint64(temp, uint64(i))
				testMap[string(temp)] = types.Counters{BytesRcvd: uint64(i), BytesSent: 0, PacketsRcvd: 0, PacketsSent: 0}
			}

			testKey := make([]byte, 8)
			binary.BigEndian.PutUint64(testKey, 1000000)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				v, exists := testMap[string(testKey)]
				globalV = v
				globalExists = exists
			}
		})
	}
}

func BenchmarkNativeMapIterator(b *testing.B) {

	testMap := make(map[string]types.Counters)
	for i := 0; i < 100000; i++ {
		temp := make([]byte, 8)
		binary.BigEndian.PutUint64(temp, uint64(i))
		testMap[string(temp)] = types.Counters{BytesRcvd: uint64(i), BytesSent: 0, PacketsRcvd: 0, PacketsSent: 0}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for k, v := range testMap {
			_ = k
			_ = v
		}
	}
}
