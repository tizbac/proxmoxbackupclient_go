package machinebackuplib

import (
	"bytes"

	"pbscommon"
)

// zeroChunkDetector recognises a full-size chunk of zeros (blank disk space:
// unpartitioned space, never-written or trimmed areas, the tail of a large
// disk) without hashing it. bytes.Equal against a zero buffer runs at several
// GB/s and stops at the first non-zero byte, while SHA-256 of 4 MiB costs
// about 10 ms on a CPU without SHA instructions; on a mostly empty disk that
// hashing dominated the run. The zero chunk's digest is worked out once per
// backup with the client's own scheme, so an encrypted snapshot (digest with
// the id_key) gets the right value too, and the images are unchanged.
type zeroChunkDetector struct {
	zeros  []byte
	digest [32]byte
}

func newZeroChunkDetector(client *pbscommon.PBSClient) zeroChunkDetector {
	zeros := make([]byte, pbscommon.PBS_FIXED_CHUNK_SIZE)
	return zeroChunkDetector{zeros: zeros, digest: client.ChunkDigest(zeros)}
}

// digestOf is hash(data), short-cut for a full-size chunk of zeros.
func (z zeroChunkDetector) digestOf(data []byte, hash func([]byte) [32]byte) [32]byte {
	if len(data) == len(z.zeros) && bytes.Equal(data, z.zeros) {
		return z.digest
	}
	return hash(data)
}
