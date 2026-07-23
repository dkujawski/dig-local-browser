package extractor

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/dkujawski/dig-local-browser/internal/simplecache"
)

var tinyPNG = []byte("\x89PNG\r\n\x1a\nsynthetic-image")

func TestExtractIdentityImage(t *testing.T) {
	source := writeCombinedEntry(t, tinyPNG, "image/png", "")
	output := t.TempDir()

	result, err := Extract(source, Options{OutputDir: output, MaxDecodedSize: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	wantHash := digest(tinyPNG)
	if result.RawSHA256 != wantHash || result.DecodedSHA256 != wantHash {
		t.Fatalf("result hashes = %+v; want %s", result, wantHash)
	}
	if result.ImageType != "png" || result.RawPath != result.ImagePath {
		t.Fatalf("result paths/type = %+v", result)
	}
	assertFile(t, result.ImagePath, tinyPNG, 0o600)
}

func TestExtractSupportedContentEncodings(t *testing.T) {
	tests := []struct {
		name     string
		encoding string
		encode   func([]byte) []byte
	}{
		{name: "gzip", encoding: "gzip", encode: gzipBytes},
		{name: "deflate", encoding: "deflate", encode: zlibBytes},
		{name: "brotli", encoding: "br", encode: brotliBytes},
		{
			name:     "stacked",
			encoding: "gzip, br",
			encode: func(data []byte) []byte {
				return brotliBytes(gzipBytes(data))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := tt.encode(tinyPNG)
			result, err := Extract(
				writeCombinedEntry(t, raw, "image/png", tt.encoding),
				Options{OutputDir: t.TempDir(), MaxDecodedSize: 1 << 20},
			)
			if err != nil {
				t.Fatal(err)
			}
			if result.RawSHA256 != digest(raw) || result.DecodedSHA256 != digest(tinyPNG) {
				t.Fatalf("result = %+v", result)
			}
			if result.RawPath == result.ImagePath {
				t.Fatalf("encoded raw and decoded paths must differ: %+v", result)
			}
			assertFile(t, result.RawPath, raw, 0o600)
			assertFile(t, result.ImagePath, tinyPNG, 0o600)
		})
	}
}

func TestExtractDeduplicatesDecodedImages(t *testing.T) {
	output := t.TempDir()
	first, err := Extract(
		writeCombinedEntry(t, gzipBytes(tinyPNG), "image/png", "gzip"),
		Options{OutputDir: output, MaxDecodedSize: 1 << 20},
	)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Extract(
		writeCombinedEntry(t, brotliBytes(tinyPNG), "image/png", "br"),
		Options{OutputDir: output, MaxDecodedSize: 1 << 20},
	)
	if err != nil {
		t.Fatal(err)
	}
	if first.ImagePath != second.ImagePath || !second.Deduplicated {
		t.Fatalf("first=%+v second=%+v", first, second)
	}
}

func TestExtractDetectsWebPLargerThanSniffBuffer(t *testing.T) {
	webp := make([]byte, 128)
	copy(webp[0:4], "RIFF")
	binary.LittleEndian.PutUint32(webp[4:8], uint32(len(webp)-8))
	copy(webp[8:12], "WEBP")

	result, err := Extract(
		writeCombinedEntry(t, webp, "image/webp", ""),
		Options{OutputDir: t.TempDir(), MaxDecodedSize: 1 << 20},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.ImageType != "webp" {
		t.Fatalf("result = %+v", result)
	}
}

func TestExtractRefusesMismatchedDigestNamedArtifact(t *testing.T) {
	output := t.TempDir()
	target := filepath.Join(output, digest(tinyPNG)+".png")
	if err := os.WriteFile(target, []byte("different content"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Extract(
		writeCombinedEntry(t, tinyPNG, "image/png", ""),
		Options{OutputDir: output, MaxDecodedSize: 1 << 20},
	)
	if err == nil || !strings.Contains(err.Error(), "refuse to replace") {
		t.Fatalf("error = %v; want no-clobber error", err)
	}
	assertFile(t, target, []byte("different content"), 0o600)
}

func TestExtractRejectsUnsafePayloads(t *testing.T) {
	tests := []struct {
		name       string
		body       []byte
		encoding   string
		maxDecoded int64
		wantError  error
	}{
		{name: "unsupported encoding", body: tinyPNG, encoding: "zstd", maxDecoded: 1 << 20, wantError: ErrUnsupportedEncoding},
		{name: "not an image", body: []byte("not an image"), maxDecoded: 1 << 20, wantError: ErrNotImage},
		{name: "decode limit", body: gzipBytes(tinyPNG), encoding: "gzip", maxDecoded: 8, wantError: ErrDecodedTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := t.TempDir()
			_, err := Extract(
				writeCombinedEntry(t, tt.body, "image/png", tt.encoding),
				Options{OutputDir: output, MaxDecodedSize: tt.maxDecoded},
			)
			if !errorsIs(err, tt.wantError) {
				t.Fatalf("error = %v; want %v", err, tt.wantError)
			}
			entries, readErr := os.ReadDir(output)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if len(entries) != 0 {
				t.Fatalf("failed extraction left artifacts: %v", entries)
			}
		})
	}
}

func TestExtractPreservesSource(t *testing.T) {
	source := writeCombinedEntry(t, tinyPNG, "image/png", "")
	before, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Extract(source, Options{OutputDir: t.TempDir(), MaxDecodedSize: 1 << 20}); err != nil {
		t.Fatal(err)
	}

	after, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	afterInfo, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) || !info.ModTime().Equal(afterInfo.ModTime()) {
		t.Fatal("source cache entry changed during extraction")
	}
}

func TestResultIsJSONSerializable(t *testing.T) {
	result, err := Extract(
		writeCombinedEntry(t, tinyPNG, "image/png", ""),
		Options{OutputDir: t.TempDir(), MaxDecodedSize: 1 << 20},
	)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"source_path", "decoded_sha256", "image_path"} {
		if !strings.Contains(string(data), `"`+field+`"`) {
			t.Fatalf("JSON %s missing %q", data, field)
		}
	}
}

func writeCombinedEntry(t *testing.T, body []byte, mimeType, encoding string) string {
	t.Helper()
	key := "https://i.redd.it/example.png"
	headers := "HTTP/1.1 200 OK\x00Content-Type: " + mimeType + "\x00"
	if encoding != "" {
		headers += "Content-Encoding: " + encoding + "\x00"
	}
	headers += "\x00"
	stream0 := responsePickle(headers)
	header := make([]byte, simplecache.FileHeaderSize)
	binary.LittleEndian.PutUint64(header[0:8], simplecache.SimpleCacheMagic)
	binary.LittleEndian.PutUint32(header[8:12], simplecache.EntryVersion)
	binary.LittleEndian.PutUint32(header[12:16], uint32(len(key)))
	eof1 := make([]byte, simplecache.FileEOFSize)
	binary.LittleEndian.PutUint64(eof1[0:8], simplecache.FinalMagic)
	eof0 := make([]byte, simplecache.FileEOFSize)
	binary.LittleEndian.PutUint64(eof0[0:8], simplecache.FinalMagic)
	binary.LittleEndian.PutUint32(eof0[16:20], uint32(len(stream0)))
	data := bytes.Join([][]byte{header, []byte(key), body, eof1, stream0, eof0}, nil)
	path := filepath.Join(t.TempDir(), "0123456789abcdef_0")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func responsePickle(headers string) []byte {
	payload := &bytes.Buffer{}
	_ = binary.Write(payload, binary.LittleEndian, uint32(3))
	_ = binary.Write(payload, binary.LittleEndian, int64(1))
	_ = binary.Write(payload, binary.LittleEndian, int64(2))
	_ = binary.Write(payload, binary.LittleEndian, uint32(len(headers)))
	payload.WriteString(headers)
	for payload.Len()%4 != 0 {
		payload.WriteByte(0)
	}
	out := &bytes.Buffer{}
	_ = binary.Write(out, binary.LittleEndian, uint32(payload.Len()))
	out.Write(payload.Bytes())
	return out.Bytes()
}

func gzipBytes(data []byte) []byte {
	var out bytes.Buffer
	writer := gzip.NewWriter(&out)
	_, _ = writer.Write(data)
	_ = writer.Close()
	return out.Bytes()
}

func zlibBytes(data []byte) []byte {
	var out bytes.Buffer
	writer := zlib.NewWriter(&out)
	_, _ = writer.Write(data)
	_ = writer.Close()
	return out.Bytes()
}

func brotliBytes(data []byte) []byte {
	var out bytes.Buffer
	writer := brotli.NewWriter(&out)
	_, _ = writer.Write(data)
	_ = writer.Close()
	return out.Bytes()
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func assertFile(t *testing.T, path string, want []byte, mode os.FileMode) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s = %x; want %x", path, got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != mode {
		t.Fatalf("%s mode = %o; want %o", path, info.Mode().Perm(), mode)
	}
}

func errorsIs(err, target error) bool {
	for err != nil {
		if err == target {
			return true
		}
		type unwrapper interface{ Unwrap() error }
		unwrapped, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = unwrapped.Unwrap()
	}
	return false
}
