package simplecache

import "encoding/binary"

// persistentHash implements Chromium base::PersistentHash, which is the
// stable SuperFastHash used by SimpleFileHeader::key_hash.
func persistentHash(data []byte) uint32 {
	if len(data) == 0 {
		return 0
	}
	hash := uint32(len(data))
	blocks := len(data) / 4
	position := 0
	for range blocks {
		hash += uint32(binary.LittleEndian.Uint16(data[position : position+2]))
		tmp := (uint32(binary.LittleEndian.Uint16(data[position+2:position+4])) << 11) ^ hash
		hash = (hash << 16) ^ tmp
		position += 4
		hash += hash >> 11
	}
	switch len(data) & 3 {
	case 3:
		hash += uint32(binary.LittleEndian.Uint16(data[position : position+2]))
		hash ^= hash << 16
		hash ^= uint32(data[position+2]) << 18
		hash += hash >> 11
	case 2:
		hash += uint32(binary.LittleEndian.Uint16(data[position : position+2]))
		hash ^= hash << 11
		hash += hash >> 17
	case 1:
		hash += uint32(data[position])
		hash ^= hash << 10
		hash += hash >> 1
	}
	hash ^= hash << 3
	hash += hash >> 5
	hash ^= hash << 4
	hash += hash >> 17
	hash ^= hash << 25
	hash += hash >> 6
	return hash
}
