// Package simplecache detects and parses Chromium Simple Cache entries.
package simplecache

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const SimpleCacheMagic uint64 = 0xfcfb6d1ba7725c30

var (
	ErrNotSimpleCache     = errors.New("not a Chromium Simple Cache entry")
	ErrUnsupportedVersion = errors.New("unsupported Simple Cache version")
	ErrCorruptEntry       = errors.New("corrupt Simple Cache entry")
	ErrTruncatedEntry     = errors.New("truncated Simple Cache entry")
)

// Detect checks the fixed little-endian magic without changing reader state.
func Detect(r io.ReaderAt, size int64) (bool, error) {
	if size < 8 {
		return false, fmt.Errorf("%w: need 8 bytes, have %d", ErrTruncatedEntry, size)
	}
	var header [8]byte
	if _, err := r.ReadAt(header[:], 0); err != nil {
		return false, fmt.Errorf("read entry magic: %w", err)
	}
	return binary.LittleEndian.Uint64(header[:]) == SimpleCacheMagic, nil
}
