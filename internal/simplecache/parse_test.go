package simplecache

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseCombinedEntry(t *testing.T) {
	key := "https://preview.redd.it/example.jpg?width=1920&format=pjpg"
	body := []byte("\xff\xd8\xffsynthetic-jpeg")
	rawHeaders := "HTTP/1.1 200 OK\x00Content-Type: image/jpeg\x00Content-Encoding: gzip\x00X-Test: one\x00X-Test: two\x00\x00"
	stream0 := buildResponsePickle(rawHeaders, true)
	data := buildCombinedFixture(key, body, stream0, true)

	entry, err := Parse(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if entry.Key != key || entry.URL != key {
		t.Fatalf("key/url = %q/%q", entry.Key, entry.URL)
	}
	if entry.Header.Version != EntryVersion || entry.HTTPStatus != 200 {
		t.Fatalf("header/status = %+v/%d", entry.Header, entry.HTTPStatus)
	}
	if entry.MIMEType != "image/jpeg" || entry.ContentEncoding != "gzip" {
		t.Fatalf("metadata = %q/%q", entry.MIMEType, entry.ContentEncoding)
	}
	if got := entry.HTTPHeaders["X-Test"]; len(got) != 2 || got[1] != "two" {
		t.Fatalf("X-Test = %v", got)
	}
	if len(entry.Streams) != 2 || entry.Streams[0].Index != 0 || entry.Streams[1].Index != 1 {
		t.Fatalf("streams = %+v", entry.Streams)
	}
	for _, stream := range entry.Streams {
		if !stream.CRCVerified {
			t.Fatalf("stream CRC not verified: %+v", stream)
		}
	}
	gotBody, err := io.ReadAll(entry.Body)
	if err != nil || !bytes.Equal(gotBody, body) {
		t.Fatalf("body = %x, %v", gotBody, err)
	}
	wantHash := sha256.Sum256(body)
	if entry.BodySHA256 != stringHex(wantHash[:]) {
		t.Fatalf("body hash = %q", entry.BodySHA256)
	}
	if entry.KeySHA256Verified == nil || !*entry.KeySHA256Verified {
		t.Fatalf("key SHA verification = %v", entry.KeySHA256Verified)
	}
}

func TestParseStream2Entry(t *testing.T) {
	data := buildStream2Fixture("https://example.test/script.js", []byte("compiled metadata"))
	entry, err := Parse(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if len(entry.Streams) != 1 || entry.Streams[0].Index != 2 || entry.Body != nil {
		t.Fatalf("entry = %+v", entry)
	}
}

func TestParseHeaderAndKeyDoesNotRequireCompleteStreams(t *testing.T) {
	key := "1/0/_dk_https://www.reddit.com https://i.redd.it/a.jpg?x=1"
	data := append(buildHeader(key), []byte(key)...)
	entry, err := ParseHeaderAndKey(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if entry.URL != "https://i.redd.it/a.jpg?x=1" {
		t.Fatalf("URL = %q", entry.URL)
	}
}

func TestParseReturnsPartialEntryForUnsupportedVersion(t *testing.T) {
	data := buildCombinedFixture("https://i.redd.it/a.jpg", []byte("body"), buildResponsePickle("HTTP/1.1 200 OK\x00\x00", false), false)
	binary.LittleEndian.PutUint32(data[8:12], EntryVersion+1)
	entry, err := Parse(bytes.NewReader(data), int64(len(data)))
	if !errors.Is(err, ErrUnsupportedVersion) || entry == nil || entry.Header.Version != EntryVersion+1 {
		t.Fatalf("entry=%+v error=%v", entry, err)
	}
}

func TestParseRejectsHostileKeyLength(t *testing.T) {
	data := make([]byte, FileHeaderSize)
	binary.LittleEndian.PutUint64(data[0:8], SimpleCacheMagic)
	binary.LittleEndian.PutUint32(data[8:12], EntryVersion)
	binary.LittleEndian.PutUint32(data[12:16], ^uint32(0))
	_, err := Parse(bytes.NewReader(data), int64(len(data)))
	if !errors.Is(err, ErrCorruptEntry) {
		t.Fatalf("error = %v; want ErrCorruptEntry", err)
	}
}

func TestParseRejectsTruncatedAndImpossibleStreams(t *testing.T) {
	if _, err := Parse(bytes.NewReader(make([]byte, 12)), 12); !errors.Is(err, ErrTruncatedEntry) {
		t.Fatalf("short error = %v", err)
	}
	data := buildCombinedFixture("https://i.redd.it/a.jpg", []byte("body"), buildResponsePickle("HTTP/1.1 200 OK\x00\x00", false), false)
	footer := len(data) - int(FileEOFSize)
	binary.LittleEndian.PutUint32(data[footer+16:footer+20], ^uint32(0))
	if _, err := Parse(bytes.NewReader(data), int64(len(data))); !errors.Is(err, ErrCorruptEntry) {
		t.Fatalf("offset error = %v", err)
	}
}

func TestParseRejectsStream1FooterSize(t *testing.T) {
	key := "https://i.redd.it/a.jpg"
	body := []byte("body")
	data := buildCombinedFixture(key, body, buildResponsePickle("HTTP/1.1 200 OK\x00\x00", false), false)
	innerFooter := int(FileHeaderSize) + len(key) + len(body)
	binary.LittleEndian.PutUint32(data[innerFooter+16:innerFooter+20], 1)
	if _, err := Parse(bytes.NewReader(data), int64(len(data))); !errors.Is(err, ErrCorruptEntry) {
		t.Fatalf("error = %v; want ErrCorruptEntry", err)
	}
}

func TestParsePreservesCRCWarning(t *testing.T) {
	data := buildCombinedFixture("https://i.redd.it/a.jpg", []byte("body"), buildResponsePickle("HTTP/1.1 200 OK\x00\x00", false), false)
	data[int(FileHeaderSize)+len("https://i.redd.it/a.jpg")] ^= 0xff
	entry, err := Parse(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if !warningsContain(entry.Warnings, "CRC32 mismatch") {
		t.Fatalf("warnings = %v", entry.Warnings)
	}
}

func TestParseFileUsesFilenameLayoutAndSetsPath(t *testing.T) {
	key := "https://i.redd.it/a.jpg"
	data := buildCombinedFixture(key, []byte("body"), buildResponsePickle("HTTP/1.1 200 OK\x00\x00", false), false)
	path := filepath.Join(t.TempDir(), "0123456789abcdef_0")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	entry, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Path != path || len(entry.Streams) != 2 {
		t.Fatalf("entry = %+v", entry)
	}

	innerFooter := int(FileHeaderSize) + len(key) + len("body")
	binary.LittleEndian.PutUint64(data[innerFooter:innerFooter+8], 0)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseFile(path); !errors.Is(err, ErrCorruptEntry) {
		t.Fatalf("corrupt _0 error = %v", err)
	}
}

func FuzzSimpleCacheParser(f *testing.F) {
	f.Add(buildCombinedFixture("https://i.redd.it/a.jpg", []byte("body"), buildResponsePickle("HTTP/1.1 200 OK\x00Content-Type: image/jpeg\x00\x00", false), true))
	f.Add([]byte("not a cache entry"))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = Parse(bytes.NewReader(data), int64(len(data)))
	})
}

func buildCombinedFixture(key string, body, stream0 []byte, keySHA bool) []byte {
	header := buildHeader(key)
	eof1 := buildEOF(body, 0, false)
	flags := uint32(flagHasCRC32)
	var keyDigest []byte
	if keySHA {
		flags |= flagHasKeySHA256
		digest := sha256.Sum256([]byte(key))
		keyDigest = digest[:]
	}
	eof0 := buildEOF(stream0, uint32(len(stream0)), false)
	binary.LittleEndian.PutUint32(eof0[8:12], flags)
	return bytes.Join([][]byte{header, []byte(key), body, eof1, stream0, keyDigest, eof0}, nil)
}

func buildStream2Fixture(key string, data []byte) []byte {
	return bytes.Join([][]byte{buildHeader(key), []byte(key), data, buildEOF(data, 0, false)}, nil)
}

func buildHeader(key string) []byte {
	header := make([]byte, FileHeaderSize)
	binary.LittleEndian.PutUint64(header[0:8], SimpleCacheMagic)
	binary.LittleEndian.PutUint32(header[8:12], EntryVersion)
	binary.LittleEndian.PutUint32(header[12:16], uint32(len(key)))
	binary.LittleEndian.PutUint32(header[16:20], persistentHash([]byte(key)))
	return header
}

func buildEOF(data []byte, streamSize uint32, corrupt bool) []byte {
	eof := make([]byte, FileEOFSize)
	binary.LittleEndian.PutUint64(eof[0:8], FinalMagic)
	binary.LittleEndian.PutUint32(eof[8:12], flagHasCRC32)
	crc := crc32.ChecksumIEEE(data)
	if corrupt {
		crc++
	}
	binary.LittleEndian.PutUint32(eof[12:16], crc)
	binary.LittleEndian.PutUint32(eof[16:20], streamSize)
	return eof
}

func buildResponsePickle(rawHeaders string, extraFlags bool) []byte {
	payload := &bytes.Buffer{}
	flags := uint32(3)
	if extraFlags {
		flags |= 1 << 31
	}
	_ = binary.Write(payload, binary.LittleEndian, flags)
	if extraFlags {
		_ = binary.Write(payload, binary.LittleEndian, uint32(1<<2))
	}
	_ = binary.Write(payload, binary.LittleEndian, int64(1))
	_ = binary.Write(payload, binary.LittleEndian, int64(2))
	if extraFlags {
		_ = binary.Write(payload, binary.LittleEndian, int64(3))
	}
	_ = binary.Write(payload, binary.LittleEndian, uint32(len(rawHeaders)))
	payload.WriteString(rawHeaders)
	for payload.Len()%4 != 0 {
		payload.WriteByte(0)
	}
	out := &bytes.Buffer{}
	_ = binary.Write(out, binary.LittleEndian, uint32(payload.Len()))
	out.Write(payload.Bytes())
	return out.Bytes()
}

func warningsContain(warnings []string, needle string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, needle) {
			return true
		}
	}
	return false
}

func stringHex(data []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, len(data)*2)
	for i, value := range data {
		out[i*2] = digits[value>>4]
		out[i*2+1] = digits[value&15]
	}
	return string(out)
}
