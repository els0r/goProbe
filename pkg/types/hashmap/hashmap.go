// package hashmap implemets a modified version of Go's map type using type
// parameters. See https://github.com/golang/go/blob/master/src/runtime/map.go
package hashmap

import (
	"github.com/zeebo/xxh3"
)

// //go:linkname runtime_memhash runtime.memhash
// // go:noescape
// func runtime_memhash(p unsafe.Pointer, seed, s uintptr) uintptr

func hash(in []byte) uint64 {
	return xxh3.Hash(in)
	// return uint64(runtime_memhash(*(*unsafe.Pointer)(unsafe.Pointer(&in)), 0, uintptr(len(in))))
}

const (
	// Maximum number of key/val pairs a bucket can hold.
	bucketCntBits = 3
	bucketCnt     = 1 << bucketCntBits

	// Maximum average load of a bucket that triggers growth is bucketCnt*13/16 (about 80% full)
	// Because of minimum alignment rules, bucketCnt is known to be at least 8.
	// Represent as loadFactorNum/loadFactorDen, to allow integer math.
	loadFactorNum = 13
	loadFactorDen = 2

	// Possible topHash values. We reserve a few possibilities for special marks.
	// Each bucket (including its overflow buckets, if any) will have either all or none of its
	// entries in the evacuated* states (except during the evacuate() method, which only happens
	// during map writes and thus no one else can observe the map during that time).

	emptyRest      = 0 // this cell is empty, and there are no more non-empty cells at higher indexes or overflows.
	emptyOne       = 1 // this cell is empty
	evacuatedX     = 2 // key/val is valid.  Entry has been evacuated to first half of larger table.
	evacuatedY     = 3 // same as above, but evacuated to second half of larger table.
	evacuatedEmpty = 4 // cell is empty, bucket is evacuated.
	minTopHash     = 5 // minimum topHash for a normal filled cell.

	// flags
	iter         = 1 // there may be an Iter using buckets
	oldIter      = 2 // there may be an Iter using oldBuckets
	sameSizeGrow = 4 // the current map growth is to a new map of the same size

	// sentinel bucket ID for Iter checks
	noCheck    = 4 << (^uintptr(0) >> 63)
	ptrBitSize = noCheck * 8
)

// Map denotes the main type of the hashmap implementation
type Map struct {
	count int

	flags     uint8
	nOverflow uint32

	buckets []bucket

	nextOverflow int

	oldBuckets *[]bucket

	keyData    []byte
	keyDataPos int

	nEvacuate int
}

type bucket struct {
	topHash [bucketCnt]uint8

	keys [bucketCnt]Key
	vals [bucketCnt]Val

	overflow *bucket
}

// NewHint instantiates a new Map with a hint as to how many valents
// will be insertVd.
func NewHint(hint int) *Map {
	if hint <= 0 {
		return &Map{keyData: make([]byte, 65536)}
	}
	nBuckets := 1
	for loadFactor(hint, nBuckets) {
		nBuckets *= 2
	}
	buckets := makeBucketArray(nBuckets)

	return &Map{buckets: buckets, nextOverflow: len(buckets), keyData: make([]byte, 65536)}
}

// Len returns the number of valents in the map
func (m *Map) Len() int {
	if m == nil {
		return 0
	}
	return m.count
}

// Get returns the valent associated with key and true if that key exists
func (m *Map) Get(key Key) (Val, bool) {
	var res Val
	_, e := m.mapaccessK(key)
	if e == nil {
		return res, false
	}
	return *e, true
}

func (m *Map) mapaccessK(key Key) (*Key, *Val) {
	if m == nil || m.count == 0 {
		return nil, nil
	}

	hash := hash(key)
	mask := m.bucketMask()
	b := &m.buckets[int(hash&mask)]
	if c := m.oldBuckets; c != nil {
		if !m.sameSizeGrow() {
			mask >>= 1
		}
		oldb := &(*c)[int(hash&mask)]
		if !evacuated(oldb) {
			b = oldb
		}
	}
	top := topHash(hash)
bucketloop:
	for ; b != nil; b = b.overflow {
		for i := uintptr(0); i < bucketCnt; i++ {
			if b.topHash[i] != top {
				if b.topHash[i] == emptyRest {
					break bucketloop
				}
				continue
			}
			if string(key) == string(b.keys[i]) {
				return &b.keys[i], &b.vals[i]
			}
		}
	}
	return nil, nil
}

// Set either creates a new entry based on the provided values or
// updates any existing valent (if exists).
func (m *Map) Set(key Key, val Val) {
	if m == nil {
		panic("Set called on nil map")
	}
	hash := hash(key)

	if m.buckets == nil {
		m.buckets = make([]bucket, 1)
		m.nextOverflow = len(m.buckets)
	}

again:
	mask := m.bucketMask()
	bucket := hash & mask
	if m.isGrowing() {
		m.growWork(int(bucket))
	}
	b := &m.buckets[hash&mask]
	top := topHash(hash)

	var insertI *uint8
	var insertK *Key
	var insertV *Val
bucketloop:
	for {
		for i := uintptr(0); i < bucketCnt; i++ {
			if b.topHash[i] != top {
				if isEmpty(b.topHash[i]) && insertI == nil {
					insertI = &b.topHash[i]
					insertK = &b.keys[i]
					insertV = &b.vals[i]
				}
				if b.topHash[i] == emptyRest {
					break bucketloop
				}
				continue
			}
			if string(key) != string(b.keys[i]) {
				continue
			}
			b.vals[i] = val
			goto done
		}
		ovf := b.overflow
		if ovf == nil {
			break
		}
		b = ovf
	}

	if !m.isGrowing() && (loadFactor(m.count+1, len(m.buckets)) ||
		tooManyOverflowBuckets(m.nOverflow, len(m.buckets))) {
		m.hashGrow()
		goto again
	}

	if insertI == nil {
		newB := m.newoverflow(b)
		insertI = &newB.topHash[0]
		insertK = &newB.keys[0]
		insertV = &newB.vals[0]
	}

	if m.keyDataPos+len(key) > len(m.keyData) {
		m.keyData = append(m.keyData, make([]byte, len(m.keyData))...)
	}
	*insertK = m.keyData[m.keyDataPos : m.keyDataPos+len(key)]
	m.keyDataPos += len(key)
	copy(*insertK, key)

	*insertV = val
	*insertI = top
	m.count++

done:
}

// SetOrUpdate either creates a new entry based on the provided values or
// updates any existing valent (if exists). This way may be very specific, but
// it avoids intermediate allocation of a value type valent in case of an update
func (m *Map) SetOrUpdate(key Key, eA, eB, eC, eD uint64) {
	if m == nil {
		panic("SetOrUpdate called on nil map")
	}
	hash := hash(key)

	if m.buckets == nil {
		m.buckets = make([]bucket, 1)
		m.nextOverflow = len(m.buckets)
	}

again:
	mask := m.bucketMask()
	bucket := hash & mask
	if m.isGrowing() {
		m.growWork(int(bucket))
	}
	b := &m.buckets[hash&mask]
	top := topHash(hash)

	var insertI *uint8
	var insertK *Key
	var insertV *Val
bucketloop:
	for {
		for i := uintptr(0); i < bucketCnt; i++ {
			if b.topHash[i] != top {
				if isEmpty(b.topHash[i]) && insertI == nil {
					insertI = &b.topHash[i]
					insertK = &b.keys[i]
					insertV = &b.vals[i]
				}
				if b.topHash[i] == emptyRest {
					break bucketloop
				}
				continue
			}
			if string(b.keys[i]) != string(key) {
				continue
			}

			b.vals[i].BytesRcvd += eA
			b.vals[i].BytesSent += eB
			b.vals[i].PacketsRcvd += eC
			b.vals[i].PacketsSent += eD
			goto done
		}
		ovf := b.overflow
		if ovf == nil {
			break
		}
		b = ovf
	}

	if !m.isGrowing() && (loadFactor(m.count+1, len(m.buckets)) ||
		tooManyOverflowBuckets(m.nOverflow, len(m.buckets))) {
		m.hashGrow()
		goto again
	}

	if insertI == nil {
		newB := m.newoverflow(b)
		insertI = &newB.topHash[0]
		insertK = &newB.keys[0]
		insertV = &newB.vals[0]
	}

	if m.keyDataPos+len(key) > len(m.keyData) {
		m.keyData = append(m.keyData, make([]byte, len(m.keyData))...)
	}
	*insertK = m.keyData[m.keyDataPos : m.keyDataPos+len(key)]
	m.keyDataPos += len(key)
	copy(*insertK, key)

	*insertV = Val{
		BytesRcvd:   eA,
		BytesSent:   eB,
		PacketsRcvd: eC,
		PacketsSent: eD,
	}
	*insertI = top
	m.count++

done:
}

// Merge allows to incorporate the content of a map m2 into an existing map m (providing
// additional in-place counter updates). It re-uses / duplicates code from the iterator
// part to minimize function call overhead and allocations
func (m *Map) Merge(m2 *Map, totals *Val) {

	var it Iter
	m2.iter(&it)

start:
	bucket := it.bucket
	b := it.bucketPtr
	i := it.i
	checkBucket := it.checkBucket

next:
	if b == nil {
		if bucket == it.startBucket && it.wrapped {
			var (
				zeroK Key
				zeroE Val
			)
			it.key = zeroK
			it.val = zeroE
			return
		}
		if m2.isGrowing() && len(it.buckets) == len(m2.buckets) {
			oldBucket := uint64(bucket) & it.m.oldBucketMask()
			b = &(*m2.oldBuckets)[oldBucket]
			if !evacuated(b) {
				checkBucket = bucket
			} else {
				b = &it.buckets[bucket]
				checkBucket = noCheck
			}
		} else {
			b = &it.buckets[bucket]
			checkBucket = noCheck
		}

		bucket++
		if bucket == len(it.buckets) {
			bucket = 0
			it.wrapped = true
		}
		i = 0
	}
	for ; i < bucketCnt; i++ {
		offi := (i + it.offset) & (bucketCnt - 1)
		if isEmpty(b.topHash[offi]) || b.topHash[offi] == evacuatedEmpty {
			continue
		}
		k := b.keys[offi]
		if checkBucket != noCheck && !m2.sameSizeGrow() {
			hash := hash(k)
			if int(hash&m2.bucketMask()) != checkBucket {
				continue
			}
		}
		if b.topHash[offi] != evacuatedX && b.topHash[offi] != evacuatedY {
			it.key = k
			it.val = b.vals[offi]
		} else {
			rk, re := m2.mapaccessK(k)
			if rk == nil {
				continue
			}
			it.key = *rk
			it.val = *re
		}
		it.bucket = bucket
		if it.bucketPtr != b {
			it.bucketPtr = b
		}
		it.i = i + 1
		it.checkBucket = checkBucket

		val := it.val
		m.SetOrUpdate(it.key, val.BytesRcvd, val.BytesSent, val.PacketsRcvd, val.PacketsSent)
		if totals != nil {
			*totals = totals.Add(val)
		}

		goto start
	}
	b = b.overflow
	i = 0
	goto next
}

// Iter instantiates an Iter to traverse the map.
func (m *Map) Iter() *Iter {
	var it Iter
	m.iter(&it)
	return &it
}

func (m *Map) iter(it *Iter) {
	if m == nil || m.count == 0 {
		return
	}
	r := uint64(1)
	it.m = m
	it.buckets = m.buckets
	it.startBucket = int(1 & m.bucketMask())
	it.bucket = it.startBucket
	it.offset = uint8(r >> (64 - bucketCntBits))

	return
}

// Clear frees as many resources as possible by making them eligible for GC
func (m *Map) Clear() {
	if m == nil || m.count == 0 {
		return
	}

	m.flags &^= sameSizeGrow
	m.nEvacuate = 0
	m.nOverflow = 0
	m.count = 0

	buckets := m.buckets[:m.nextOverflow]
	for i := range buckets {
		buckets[i] = bucket{}
	}

	m.ClearFast()
}

// ClearFast nils all main resources, making them eligible for GC (but
// probably not as effectively as Clear())
func (m *Map) ClearFast() {
	m.oldBuckets = nil
	m.keyData = nil
	m = nil
}

func (m *Map) hashGrow() {
	newSize := len(m.buckets) * 2
	if !loadFactor(m.count+1, len(m.buckets)) {
		newSize = len(m.buckets)
		m.flags |= sameSizeGrow
	}
	oldBuckets := m.buckets
	newBuckets := makeBucketArray(newSize)

	flags := m.flags &^ (iter | oldIter)
	if m.flags&iter != 0 {
		flags |= oldIter
	}

	m.flags = flags
	m.oldBuckets = &oldBuckets
	m.buckets = newBuckets
	m.nextOverflow = len(m.buckets)
	m.nEvacuate = 0
	m.nOverflow = 0
}

func (m *Map) newoverflow(b *bucket) *bucket {
	if m.nextOverflow < cap(m.buckets) {
		b.overflow = &m.buckets[:cap(m.buckets)][m.nextOverflow]
		m.nextOverflow++
	} else {
		b.overflow = &bucket{}
	}
	m.nOverflow++
	return b.overflow
}

func (m *Map) isGrowing() bool {
	return m.oldBuckets != nil
}

func (m *Map) sameSizeGrow() bool {
	return m.flags&sameSizeGrow != 0
}

func (m *Map) bucketMask() uint64 {
	return uint64(len(m.buckets) - 1)
}

func (m *Map) oldBucketMask() uint64 {
	return uint64(len(*m.oldBuckets) - 1)
}

func (m *Map) growWork(bucket int) {
	m.evacuate(int(uint64(bucket) & m.oldBucketMask()))
	if m.isGrowing() {
		m.evacuate(m.nEvacuate)
	}
}

func (m *Map) bucketEvacuated(bucket uint64) bool {
	return evacuated(&(*m.oldBuckets)[bucket])
}

type evacDst struct {
	b *bucket
	i int
}

func (m *Map) evacuate(oldBucket int) {
	b := &(*m.oldBuckets)[oldBucket]
	newBit := len(*m.oldBuckets)
	if !evacuated(b) {

		var xy [2]evacDst
		x := &xy[0]
		x.b = &m.buckets[oldBucket]

		if !m.sameSizeGrow() {
			y := &xy[1]
			y.b = &m.buckets[oldBucket+newBit]
		}

		for ; b != nil; b = b.overflow {
			for i := 0; i < bucketCnt; i++ {
				top := b.topHash[i]
				if isEmpty(top) {
					b.topHash[i] = evacuatedEmpty
					continue
				}
				if top < minTopHash {
					panic("bad map state")
				}
				var useY uint8
				if !m.sameSizeGrow() {
					hash := hash(b.keys[i])
					if hash&uint64(newBit) != 0 {
						useY = 1
					}
				}

				if evacuatedX+1 != evacuatedY || evacuatedX^1 != evacuatedY {
					panic("bad evacuatedN")
				}

				b.topHash[i] = evacuatedX + useY
				dst := &xy[useY]

				if dst.i == bucketCnt {
					dst.b = m.newoverflow(dst.b)
					dst.i = 0
				}

				dst.b.topHash[dst.i&(bucketCnt-1)] = top
				dst.b.keys[dst.i&(bucketCnt-1)] = b.keys[i]
				dst.b.vals[dst.i&(bucketCnt-1)] = b.vals[i]
				dst.i++
			}
		}

		if m.flags&oldIter == 0 {
			b := &(*m.oldBuckets)[oldBucket]
			b.keys = [bucketCnt]Key{}
			b.vals = [bucketCnt]Val{}
			b.overflow = nil
		}
	}

	if oldBucket == m.nEvacuate {
		m.advanceEvacuationMark(newBit)
	}
}

func (m *Map) advanceEvacuationMark(newBit int) {
	m.nEvacuate++

	stop := m.nEvacuate + 1024
	if stop > newBit {
		stop = newBit
	}
	for m.nEvacuate != stop && m.bucketEvacuated(uint64(m.nEvacuate)) {
		m.nEvacuate++
	}
	if m.nEvacuate == newBit {
		m.oldBuckets = nil
		m.flags &^= sameSizeGrow
	}
}

func topHash(hash uint64) uint8 {
	top := uint8(hash >> (ptrBitSize - 8))
	if top < minTopHash {
		top += minTopHash
	}
	return top
}

func evacuated(b *bucket) bool {
	h := b.topHash[0]
	return h > emptyOne && h < minTopHash
}

func makeBucketArray(nBuckets int) []bucket {

	if nBuckets&(nBuckets-1) != 0 {
		panic("invalid number of buckets")
	}
	var newBuckets []bucket

	toAdd := nBuckets >> 4
	if toAdd == 0 {
		newBuckets = make([]bucket, nBuckets)
	} else {
		newBuckets = append([]bucket(nil),
			make([]bucket, nBuckets+toAdd)...)
		newBuckets = newBuckets[:nBuckets]
	}

	return newBuckets
}

func loadFactor(count int, nBuckets int) bool {
	return count > bucketCnt && uint64(count) > loadFactorNum*(uint64(nBuckets)/loadFactorDen)
}

func tooManyOverflowBuckets(nOverflow uint32, nBuckets int) bool {
	return nOverflow >= uint32(nBuckets)
}

func isEmpty(x uint8) bool {
	return x <= emptyOne
}
