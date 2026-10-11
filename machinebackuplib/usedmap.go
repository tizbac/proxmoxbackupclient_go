package machinebackuplib

// usedMap says which parts of an NTFS volume hold data: one bit per
// cluster, set when the file system uses it. Free space is not read in a
// machine backup; it goes into the image as zeros, which cost nothing to
// store or send after the first such chunk.
type usedMap struct {
	clusterSize int64
	bits        []byte
	clusters    int64
}

// anyUsed reports whether any cluster in [off, off+n) of the volume is used.
// Space past the clusters the bitmap covers counts as used (read as before).
func (m *usedMap) anyUsed(off, n int64) bool {
	first := off / m.clusterSize
	last := (off + n - 1) / m.clusterSize
	if last >= m.clusters {
		return true
	}
	for c := first; c <= last; {
		// Whole bytes at a time where possible.
		if c%8 == 0 && c+8 <= last+1 {
			if m.bits[c/8] != 0 {
				return true
			}
			c += 8
			continue
		}
		if m.bits[c/8]&(1<<(c%8)) != 0 {
			return true
		}
		c++
	}
	return false
}

// zeroFree zeroes the bytes of data (read from volume offset off) that lie
// in free clusters.
func (m *usedMap) zeroFree(data []byte, off int64) {
	end := off + int64(len(data))
	for c := off / m.clusterSize; c*m.clusterSize < end && c < m.clusters; c++ {
		if m.bits[c/8]&(1<<(c%8)) != 0 {
			continue
		}
		from, to := c*m.clusterSize, (c+1)*m.clusterSize
		if from < off {
			from = off
		}
		if to > end {
			to = end
		}
		clear(data[from-off : to-off])
	}
}

// free clears the clusters [lcn, lcn+count): they are read as zeros.
func (m *usedMap) free(lcn, count int64) {
	for c := lcn; c < lcn+count && c < m.clusters; c++ {
		if c >= 0 {
			m.bits[c/8] &^= 1 << (c % 8)
		}
	}
}

// UsedMap is usedMap for callers outside the package (tests of the Linux
// free-space maps in the GUI module).
type UsedMap = usedMap

// ZeroFree zeroes the bytes of data, read from volume offset off, that lie
// in free space.
func (m *usedMap) ZeroFree(data []byte, off int64) { m.zeroFree(data, off) }

// UsedBytes is how much of the volume the map marks used.
func (m *usedMap) UsedBytes() int64 {
	var n int64
	for c := int64(0); c < m.clusters; c++ {
		if m.bits[c/8]&(1<<(c%8)) != 0 {
			n++
		}
	}
	return n * m.clusterSize
}

// FreeBytes marks the byte range [off, off+n) of the volume free (the
// clusters wholly inside it).
func (m *usedMap) FreeBytes(off, n int64) {
	first := (off + m.clusterSize - 1) / m.clusterSize
	last := (off + n) / m.clusterSize // exclusive
	if last > first {
		m.free(first, last-first)
	}
}

// newUsedMap is a map of clusters clusters of clusterSize bytes, all used.
func newUsedMap(clusterSize, clusters int64) *usedMap {
	m := &usedMap{clusterSize: clusterSize, clusters: clusters, bits: make([]byte, (clusters+7)/8)}
	for i := range m.bits {
		m.bits[i] = 0xFF
	}
	return m
}

// markUsed sets clusters [c, c+count) used.
func (m *usedMap) markUsed(c, count int64) {
	for i := c; i < c+count && i < m.clusters; i++ {
		if i >= 0 {
			m.bits[i/8] |= 1 << (i % 8)
		}
	}
}
