package simplecache

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxKeyLength       int64 = 1024 * 1024
	maxMetadataLength  int64 = 16 * 1024 * 1024
	maxValidationBytes int64 = 64 * 1024 * 1024
)

type fileEOF struct {
	flags      uint32
	crc32      uint32
	streamSize uint32
}

type layout int

const (
	layoutAuto layout = iota
	layoutCombined
	layoutStream2
)

// Parse safely parses a Simple Cache entry file. It auto-detects the combined
// stream-0/1 layout and otherwise treats a structurally valid file as stream 2.
func Parse(r io.ReaderAt, size int64) (*Entry, error) {
	return parseWithLayout(r, size, layoutAuto)
}

// ParseHeaderAndKey validates and reads only the fixed header and bounded key.
// It is intended for discovery scoring and does not require complete streams.
func ParseHeaderAndKey(r io.ReaderAt, size int64) (*Entry, error) {
	entry, _, err := parseHeaderAndKey(r, size)
	return entry, err
}

// ParseFile opens path read-only and uses Chromium's filename suffix to avoid
// reinterpreting a corrupt combined-stream file as a stream-2 file.
func ParseFile(path string) (*Entry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open cache entry %q: %w", path, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat cache entry %q: %w", path, err)
	}
	entry, parseErr := ParseFileReader(path, file, info.Size())
	return entry, parseErr
}

// ParseFileReader parses an already-open entry using path's Chromium filename
// suffix to enforce its layout. The caller retains ownership of r.
func ParseFileReader(path string, r io.ReaderAt, size int64) (*Entry, error) {
	mode := layoutAuto
	switch strings.ToLower(filepath.Ext(path)) {
	case "":
		base := strings.ToLower(filepath.Base(path))
		if strings.HasSuffix(base, "_0") {
			mode = layoutCombined
		} else if strings.HasSuffix(base, "_1") {
			mode = layoutStream2
		}
	}
	entry, parseErr := parseWithLayout(r, size, mode)
	if entry != nil {
		entry.Path = path
	}
	return entry, parseErr
}

func parseWithLayout(r io.ReaderAt, size int64, mode layout) (*Entry, error) {
	entry, baseOffset, err := parseHeaderAndKey(r, size)
	if err != nil {
		return entry, err
	}
	lastEOF, err := readEOF(r, size-FileEOFSize, size)
	if err != nil {
		return entry, err
	}
	if lastEOF.streamSize > math.MaxInt32 {
		return entry, fmt.Errorf("%w: stream 0 length %d exceeds int32", ErrCorruptEntry, lastEOF.streamSize)
	}
	if mode == layoutStream2 {
		if err := parseStream2(r, size, baseOffset, lastEOF, entry); err != nil {
			return entry, err
		}
		return entry, nil
	}
	combinedErr := parseCombined(r, size, baseOffset, lastEOF, entry)
	if combinedErr == nil {
		return entry, nil
	}
	if mode == layoutCombined || lastEOF.streamSize != 0 {
		return entry, combinedErr
	}
	if err := parseStream2(r, size, baseOffset, lastEOF, entry); err != nil {
		return entry, errors.Join(combinedErr, err)
	}
	entry.Warnings = append(entry.Warnings, "combined stream layout absent; interpreted as stream 2")
	return entry, nil
}

func parseHeaderAndKey(r io.ReaderAt, size int64) (*Entry, int64, error) {
	if size < FileHeaderSize {
		return nil, 0, fmt.Errorf("%w: need %d-byte header, have %d", ErrTruncatedEntry, FileHeaderSize, size)
	}
	headerBytes, err := readAt(r, 0, FileHeaderSize, size)
	if err != nil {
		return nil, 0, err
	}
	header := EntryHeader{
		InitialMagic: binary.LittleEndian.Uint64(headerBytes[0:8]),
		Version:      binary.LittleEndian.Uint32(headerBytes[8:12]),
		KeyLength:    binary.LittleEndian.Uint32(headerBytes[12:16]),
		KeyHash:      binary.LittleEndian.Uint32(headerBytes[16:20]),
	}
	entry := &Entry{Header: header, HTTPHeaders: make(map[string][]string), Warnings: []string{}}
	if header.InitialMagic != SimpleCacheMagic {
		return entry, 0, ErrNotSimpleCache
	}
	if header.Version != EntryVersion {
		return entry, 0, fmt.Errorf("%w: entry version %d, supported %d", ErrUnsupportedVersion, header.Version, EntryVersion)
	}
	keyLength := int64(header.KeyLength)
	if keyLength > maxKeyLength {
		return entry, 0, fmt.Errorf("%w: key length %d exceeds %d", ErrCorruptEntry, keyLength, maxKeyLength)
	}
	baseOffset, ok := checkedAdd(FileHeaderSize, keyLength)
	if !ok || baseOffset > size {
		return entry, 0, fmt.Errorf("%w: key extends beyond entry", ErrTruncatedEntry)
	}
	keyBytes, err := readAt(r, FileHeaderSize, keyLength, size)
	if err != nil {
		return entry, 0, err
	}
	entry.Key = string(keyBytes)
	entry.URL = extractURL(entry.Key)
	entry.KeyHashVerified = persistentHash(keyBytes) == header.KeyHash
	if !entry.KeyHashVerified {
		entry.Warnings = append(entry.Warnings, "cache key PersistentHash mismatch")
	}
	return entry, baseOffset, nil
}

func parseCombined(r io.ReaderAt, size, baseOffset int64, lastEOF fileEOF, entry *Entry) error {
	shaLength := int64(0)
	if lastEOF.flags&flagHasKeySHA256 != 0 {
		shaLength = sha256.Size
	}
	stream0Length := int64(lastEOF.streamSize)
	fixed, ok := checkedAdd(2*FileEOFSize, stream0Length)
	if !ok {
		return fmt.Errorf("%w: combined stream lengths overflow", ErrCorruptEntry)
	}
	fixed, ok = checkedAdd(fixed, shaLength)
	if !ok || fixed > size-baseOffset {
		return fmt.Errorf("%w: combined stream lengths exceed file", ErrCorruptEntry)
	}
	stream1Length := size - baseOffset - fixed
	stream1EOFOffset, ok := checkedAdd(baseOffset, stream1Length)
	if !ok {
		return fmt.Errorf("%w: stream 1 footer offset overflow", ErrCorruptEntry)
	}
	stream1EOF, err := readEOF(r, stream1EOFOffset, size)
	if err != nil {
		return fmt.Errorf("%w: stream 1 footer: %v", ErrCorruptEntry, err)
	}
	if stream1EOF.streamSize != 0 {
		return fmt.Errorf("%w: stream 1 footer has nonzero stream_size %d", ErrCorruptEntry, stream1EOF.streamSize)
	}
	appendUnknownFlagWarning(lastEOF.flags, 0, entry)
	appendUnknownFlagWarning(stream1EOF.flags, 1, entry)
	stream0Offset, ok := checkedAdd(stream1EOFOffset, FileEOFSize)
	if !ok {
		return fmt.Errorf("%w: stream 0 offset overflow", ErrCorruptEntry)
	}
	stream0 := Stream{Index: 0, Offset: stream0Offset, Length: stream0Length, EOFOffset: size - FileEOFSize, HasCRC32: lastEOF.flags&flagHasCRC32 != 0, ExpectedCRC32: lastEOF.crc32}
	stream1 := Stream{Index: 1, Offset: baseOffset, Length: stream1Length, EOFOffset: stream1EOFOffset, HasCRC32: stream1EOF.flags&flagHasCRC32 != 0, ExpectedCRC32: stream1EOF.crc32}
	validateStream(r, &stream0, entry)
	validateStream(r, &stream1, entry)
	entry.Streams = []Stream{stream0, stream1}
	entry.Body = io.NewSectionReader(r, stream1.Offset, stream1.Length)
	entry.BodySHA256 = digestSection(r, stream1, entry)
	if shaLength != 0 {
		verifyKeySHA256(r, stream0Offset+stream0Length, size, entry)
	}
	parseHTTPStream(r, stream0, entry)
	return nil
}

func parseStream2(r io.ReaderAt, size, baseOffset int64, eof fileEOF, entry *Entry) error {
	if eof.streamSize != 0 {
		return fmt.Errorf("%w: stream 2 footer has nonzero stream_size %d", ErrCorruptEntry, eof.streamSize)
	}
	appendUnknownFlagWarning(eof.flags, 2, entry)
	length := size - baseOffset - FileEOFSize
	if length < 0 {
		return fmt.Errorf("%w: stream 2 has negative length", ErrCorruptEntry)
	}
	stream := Stream{Index: 2, Offset: baseOffset, Length: length, EOFOffset: size - FileEOFSize, HasCRC32: eof.flags&flagHasCRC32 != 0, ExpectedCRC32: eof.crc32}
	validateStream(r, &stream, entry)
	entry.Streams = []Stream{stream}
	return nil
}

func appendUnknownFlagWarning(flags uint32, stream int, entry *Entry) {
	if unknown := flags & ^(flagHasCRC32 | flagHasKeySHA256); unknown != 0 {
		entry.Warnings = append(entry.Warnings, fmt.Sprintf("stream %d footer has unknown flags %08x", stream, unknown))
	}
}

func readEOF(r io.ReaderAt, offset, size int64) (fileEOF, error) {
	var out fileEOF
	data, err := readAt(r, offset, FileEOFSize, size)
	if err != nil {
		return out, err
	}
	if binary.LittleEndian.Uint64(data[0:8]) != FinalMagic {
		return out, fmt.Errorf("%w: footer magic mismatch at offset %d", ErrCorruptEntry, offset)
	}
	out.flags = binary.LittleEndian.Uint32(data[8:12])
	out.crc32 = binary.LittleEndian.Uint32(data[12:16])
	out.streamSize = binary.LittleEndian.Uint32(data[16:20])
	return out, nil
}

func validateStream(r io.ReaderAt, stream *Stream, entry *Entry) {
	if !stream.HasCRC32 {
		return
	}
	if stream.Length > maxValidationBytes {
		entry.Warnings = append(entry.Warnings, fmt.Sprintf("stream %d CRC32 validation skipped: length %d exceeds limit", stream.Index, stream.Length))
		return
	}
	hash := crc32.NewIEEE()
	if _, err := io.Copy(hash, io.NewSectionReader(r, stream.Offset, stream.Length)); err != nil {
		entry.Warnings = append(entry.Warnings, fmt.Sprintf("stream %d CRC32 read failed: %v", stream.Index, err))
		return
	}
	actual := hash.Sum32()
	stream.CRCVerified = actual == stream.ExpectedCRC32
	if !stream.CRCVerified {
		entry.Warnings = append(entry.Warnings, fmt.Sprintf("stream %d CRC32 mismatch: expected %08x, got %08x", stream.Index, stream.ExpectedCRC32, actual))
	}
}

func digestSection(r io.ReaderAt, stream Stream, entry *Entry) string {
	if stream.Length > maxValidationBytes {
		entry.Warnings = append(entry.Warnings, fmt.Sprintf("stream %d SHA-256 skipped: length %d exceeds limit", stream.Index, stream.Length))
		return ""
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, io.NewSectionReader(r, stream.Offset, stream.Length)); err != nil {
		entry.Warnings = append(entry.Warnings, fmt.Sprintf("stream %d SHA-256 read failed: %v", stream.Index, err))
		return ""
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func verifyKeySHA256(r io.ReaderAt, offset, size int64, entry *Entry) {
	stored, err := readAt(r, offset, sha256.Size, size)
	if err != nil {
		entry.Warnings = append(entry.Warnings, "key SHA-256 could not be read")
		return
	}
	digest := sha256.Sum256([]byte(entry.Key))
	verified := string(stored) == string(digest[:])
	entry.KeySHA256Verified = &verified
	if !verified {
		entry.Warnings = append(entry.Warnings, "cache key SHA-256 mismatch")
	}
}

func parseHTTPStream(r io.ReaderAt, stream Stream, entry *Entry) {
	if stream.Length == 0 {
		entry.Warnings = append(entry.Warnings, "stream 0 is empty; HTTP metadata unavailable")
		return
	}
	if stream.Length > maxMetadataLength {
		entry.Warnings = append(entry.Warnings, fmt.Sprintf("HTTP metadata skipped: stream 0 length %d exceeds limit", stream.Length))
		return
	}
	data, err := readAt(r, stream.Offset, stream.Length, stream.Offset+stream.Length)
	if err != nil {
		entry.Warnings = append(entry.Warnings, fmt.Sprintf("HTTP metadata read failed: %v", err))
		return
	}
	metadata, err := parseResponseMetadata(data)
	if err != nil {
		entry.Warnings = append(entry.Warnings, fmt.Sprintf("HTTP metadata parse failed: %v", err))
		return
	}
	entry.HTTPStatusLine = metadata.statusLine
	entry.HTTPStatus = metadata.status
	entry.HTTPHeaders = metadata.headers
	if values := metadata.headers["Content-Type"]; len(values) > 0 {
		entry.MIMEType = strings.ToLower(strings.TrimSpace(strings.SplitN(values[0], ";", 2)[0]))
	}
	if values := metadata.headers["Content-Encoding"]; len(values) > 0 {
		entry.ContentEncoding = strings.ToLower(strings.TrimSpace(values[0]))
	}
}

func readAt(r io.ReaderAt, offset, length, size int64) ([]byte, error) {
	if offset < 0 || length < 0 || offset > size || length > size-offset {
		return nil, fmt.Errorf("%w: range offset=%d length=%d size=%d", ErrTruncatedEntry, offset, length, size)
	}
	if length > int64(int(^uint(0)>>1)) {
		return nil, fmt.Errorf("%w: range length %d cannot be allocated", ErrCorruptEntry, length)
	}
	data := make([]byte, int(length))
	if _, err := r.ReadAt(data, offset); err != nil {
		return nil, fmt.Errorf("%w: read offset=%d length=%d: %v", ErrTruncatedEntry, offset, length, err)
	}
	return data, nil
}

func checkedAdd(a, b int64) (int64, bool) {
	if b > 0 && a > math.MaxInt64-b || b < 0 && a < math.MinInt64-b {
		return 0, false
	}
	return a + b, true
}

func extractURL(key string) string {
	for _, scheme := range []string{"https://", "http://"} {
		for searchTo := len(key); searchTo > 0; {
			index := strings.LastIndex(key[:searchTo], scheme)
			if index < 0 {
				break
			}
			candidate := strings.TrimSpace(key[index:])
			if parsed, err := url.Parse(candidate); err == nil && parsed.Scheme != "" && parsed.Host != "" {
				return candidate
			}
			searchTo = index
		}
	}
	return ""
}
