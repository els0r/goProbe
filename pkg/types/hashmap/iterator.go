package hashmap

import "github.com/zeebo/xxh3"

// Iter provides a map Iter to allow traversal
type Iter struct {
	key         Key
	val         Val
	m           *Map
	buckets     []bucket
	bucketPtr   *bucket
	startBucket int
	offset      uint8
	wrapped     bool
	i           uint8
	bucket      int
	checkBucket int
}

// Key returns the key at the current iterator position
func (it *Iter) Key() Key {
	return it.key
}

// Val returns the value / valent at the current iterator position
func (it *Iter) Val() Val {
	return it.val
}

// Next updates the iterator to the next element (returning false if none exists)
func (it *Iter) Next() bool {
	m := it.m
	if m == nil {
		return false
	}
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
			return false
		}
		if m.isGrowing() && len(it.buckets) == len(m.buckets) {
			oldBucket := uint64(bucket) & it.m.oldBucketMask()
			b = &(*m.oldBuckets)[oldBucket]
			if !evacuated(b) {
				checkBucket = bucket
			} else {
				b = &it.buckets[bucket]
				checkBucket = noBucket
			}
		} else {
			b = &it.buckets[bucket]
			checkBucket = noBucket
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
		if checkBucket != noBucket && !m.sameSizeGrow() {
			hash := xxh3.Hash(k)
			if int(hash&m.bucketMask()) != checkBucket {
				continue
			}
		}
		if b.topHash[offi] != evacuatedX && b.topHash[offi] != evacuatedY {
			it.key = k
			it.val = b.vals[offi]
		} else {
			rk, re := m.mapaccessK(k)
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
		return true
	}
	b = b.overflow
	i = 0
	goto next
}

// MetaIter denotes a wrapper around an IPv4 + IPv6 hashmap construct, allowing easy
// access to a global iterator running through both sub-maps
type MetaIter struct {
	*Iter

	v6Iter *Iter // secondary iterator
}

// Next updates the iterator to the next element (returning false if none exists)
func (i *MetaIter) Next() bool {

	// Attempt to advance the current iterator
	iter := i.Iter.Next()
	if iter {
		return iter
	}

	// If there was no next element, skip to the v6 iterator (if exists)
	if i.v6Iter != nil {

		// Nil the v6 iterator (to prevent repeated access) and return
		// whatever the iterator provides
		i.Iter = i.v6Iter
		i.v6Iter = nil
		return i.Iter.Next()
	}

	// No more items left on either iterator
	return false
}
