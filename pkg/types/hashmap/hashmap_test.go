package hashmap

import (
	"encoding/binary"
	"testing"

	"github.com/els0r/goProbe/pkg/types"
	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/require"
)

func TestSimpleHashMapOperations(t *testing.T) {

	testMap := New()
	testMap.Set([]byte("a"), types.Counters{BytesRcvd: 0, BytesSent: 1, PacketsRcvd: 0, PacketsSent: 0})

	val, exists := testMap.Get([]byte("a"))
	require.True(t, exists)
	require.Equal(t, types.Counters{BytesRcvd: 0, BytesSent: 1, PacketsRcvd: 0, PacketsSent: 0}, val)

	val, exists = testMap.Get([]byte("b"))
	require.False(t, exists)
	require.Equal(t, types.Counters{BytesRcvd: 0, BytesSent: 0, PacketsRcvd: 0, PacketsSent: 0}, val)

	for i := testMap.Iter(); i.Next(); {
		t.Log(i.Key(), i.Val())
	}
}

func TestHashMapSetOrUpdate(t *testing.T) {

	testMap := New()
	testMap.Set([]byte("a"), types.Counters{BytesRcvd: 0, BytesSent: 1, PacketsRcvd: 0, PacketsSent: 0})

	val, exists := testMap.Get([]byte("a"))
	require.True(t, exists)
	require.Equal(t, types.Counters{BytesRcvd: 0, BytesSent: 1, PacketsRcvd: 0, PacketsSent: 0}, val)

	testMap.SetOrUpdate([]byte("a"), 0, 2, 0, 1)

	val, exists = testMap.Get([]byte("a"))
	require.True(t, exists)
	require.Equal(t, types.Counters{BytesRcvd: 0, BytesSent: 3, PacketsRcvd: 0, PacketsSent: 1}, val)

	for i := testMap.Iter(); i.Next(); {
		t.Log(i.Key(), i.Val())
	}

}

func TestLinearHashMapOperations(t *testing.T) {

	testMap := New()
	for i := 0; i < 100000; i++ {
		temp := make([]byte, 8)
		binary.BigEndian.PutUint64(temp, uint64(i))
		testMap.Set(temp, types.Counters{BytesRcvd: uint64(i), BytesSent: 0, PacketsRcvd: 0, PacketsSent: 0})
	}

	require.Equal(t, 100000, int(testMap.Len()))

	for i := 0; i < 100000; i++ {
		temp := make([]byte, 8)
		binary.BigEndian.PutUint64(temp, uint64(i))
		val, exists := testMap.Get(temp)
		require.True(t, exists)
		require.Equal(t, types.Counters{BytesRcvd: uint64(i), BytesSent: 0, PacketsRcvd: 0, PacketsSent: 0}, val)

	}
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

func BenchmarkNativeMapAccess(b *testing.B) {

	testMap := New()
	for i := 0; i < 1000000; i++ {
		var ip [16]byte
		binary.BigEndian.PutUint64(ip[:], uint64(i))

		testMap.Set(types.NewKey(ip[:], ip[:], []byte{0, 0}, 0), types.Counters{
			BytesRcvd:   uint64(i),
			BytesSent:   uint64(i),
			PacketsRcvd: uint64(i),
			PacketsSent: uint64(i),
		})
	}

	b.ReportAllocs()
	b.ResetTimer()

	checkKey := types.Key{}

	for i := 0; i < b.N; i++ {
		_, _ = testMap.Get(checkKey)
	}
}

type flatByteArrayKey [35]byte

func BenchmarkByteArrayMapAccess(b *testing.B) {

	testMap := make(map[flatByteArrayKey]*types.Counters)
	for i := 0; i < 1000000; i++ {
		var key flatByteArrayKey
		binary.BigEndian.PutUint64(key[:8], uint64(i))
		binary.BigEndian.PutUint64(key[8:16], uint64(i))

		testMap[key] = &types.Counters{
			BytesRcvd:   uint64(i),
			BytesSent:   uint64(i),
			PacketsRcvd: uint64(i),
			PacketsSent: uint64(i),
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

	testMap := make(map[string]*types.Counters)
	for i := 0; i < 1000000; i++ {
		var key flatByteArrayKey
		binary.BigEndian.PutUint64(key[:8], uint64(i))
		binary.BigEndian.PutUint64(key[8:16], uint64(i))

		testMap[string(key[:])] = &types.Counters{
			BytesRcvd:   uint64(i),
			BytesSent:   uint64(i),
			PacketsRcvd: uint64(i),
			PacketsSent: uint64(i),
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

	testMap := make(map[string]*types.Counters)
	for i := 0; i < 1000000; i++ {
		var key = make([]byte, 35)
		binary.BigEndian.PutUint64(key[:8], uint64(i))
		binary.BigEndian.PutUint64(key[8:16], uint64(i))

		testMap[string(key)] = &types.Counters{
			BytesRcvd:   uint64(i),
			BytesSent:   uint64(i),
			PacketsRcvd: uint64(i),
			PacketsSent: uint64(i),
		}
	}

	b.ReportAllocs()
	b.ResetTimer()

	checkKey := make([]byte, 35)

	for i := 0; i < b.N; i++ {
		_ = testMap[string(checkKey)]
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

func BenchmarkHashMapAccesses(b *testing.B) {

	testMap := New()
	for i := 0; i < 100000; i++ {
		temp := make([]byte, 8)
		binary.BigEndian.PutUint64(temp, uint64(i))
		testMap.Set(temp, types.Counters{BytesRcvd: uint64(i), BytesSent: 0, PacketsRcvd: 0, PacketsSent: 0})
	}

	var res types.Counters
	var ex bool
	testKey := make([]byte, 8)
	binary.BigEndian.PutUint64(testKey, 42)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v, exists := testMap.Get(testKey)
		res = v
		ex = exists
		_ = res
		_ = ex
	}
}

func BenchmarkHashMapMisses(b *testing.B) {

	testMap := New()
	for i := 0; i < 100000; i++ {
		temp := make([]byte, 8)
		binary.BigEndian.PutUint64(temp, uint64(i))
		testMap.Set(temp, types.Counters{BytesRcvd: uint64(i), BytesSent: 0, PacketsRcvd: 0, PacketsSent: 0})
	}

	var res types.Counters
	var ex bool
	testKey := make([]byte, 8)
	binary.BigEndian.PutUint64(testKey, 1000000)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v, exists := testMap.Get(testKey)
		res = v
		ex = exists
		_ = res
		_ = ex
	}

}

func BenchmarkHashMapNativeAccesses(b *testing.B) {

	testMap := make(map[string]types.Counters)
	for i := 0; i < 100000; i++ {
		temp := make([]byte, 8)
		binary.BigEndian.PutUint64(temp, uint64(i))
		testMap[string(temp)] = types.Counters{BytesRcvd: uint64(i), BytesSent: 0, PacketsRcvd: 0, PacketsSent: 0}
	}

	testKey := make([]byte, 8)
	binary.BigEndian.PutUint64(testKey, 42)
	var res types.Counters
	var ex bool

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v, exists := testMap[string(testKey)]
		res = v
		ex = exists
		_ = res
		_ = ex
	}
}

func BenchmarkHashMapNativeMisses(b *testing.B) {

	testMap := make(map[string]types.Counters)
	for i := 0; i < 100000; i++ {
		temp := make([]byte, 8)
		binary.BigEndian.PutUint64(temp, uint64(i))
		testMap[string(temp)] = types.Counters{BytesRcvd: uint64(i), BytesSent: 0, PacketsRcvd: 0, PacketsSent: 0}
	}

	testKey := make([]byte, 8)
	binary.BigEndian.PutUint64(testKey, 1000000)
	var res types.Counters
	var ex bool

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v, exists := testMap[string(testKey)]
		res = v
		ex = exists
		_ = res
		_ = ex
	}
}
