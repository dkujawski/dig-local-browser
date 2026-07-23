package cli

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dkujawski/dig-local-browser/internal/extractor"
	"github.com/dkujawski/dig-local-browser/internal/manifest"
	"github.com/dkujawski/dig-local-browser/internal/simplecache"
)

func TestExtractWritesJSONResultAndArtifacts(t *testing.T) {
	image := []byte("\x89PNG\r\n\x1a\ncli-image")
	source := writeExtractEntry(t, gzipExtractBytes(image), "gzip")
	output := t.TempDir()
	var stdout, stderr bytes.Buffer

	exit := Run(
		context.Background(),
		[]string{"extract", source, "--output", output, "--max-decoded-size", "1MiB"},
		&stdout,
		&stderr,
	)

	if exit != ExitSuccess {
		t.Fatalf("exit=%d stderr=%q", exit, stderr.String())
	}
	var result extractor.Result
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &result); err != nil {
		t.Fatalf("stdout is not one JSON result: %q: %v", stdout.String(), err)
	}
	if result.SourcePath != source || result.ImageType != "png" {
		t.Fatalf("result = %+v", result)
	}
	if _, err := os.Stat(result.RawPath); err != nil {
		t.Fatalf("raw artifact: %v", err)
	}
	if got, err := os.ReadFile(result.ImagePath); err != nil || !bytes.Equal(got, image) {
		t.Fatalf("decoded artifact = %x, %v", got, err)
	}
}

func TestExtractReadsJSONLAndContinuesAfterErrors(t *testing.T) {
	image := []byte("\x89PNG\r\n\x1a\nbatch-image")
	source := writeExtractEntry(t, image, "")
	record, err := json.Marshal(manifest.Candidate{Path: source})
	if err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(t.TempDir(), "findings.jsonl")
	data := append([]byte("{not json}\n"), append(record, '\n')...)
	if err := os.WriteFile(input, data, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	exit := Run(
		context.Background(),
		[]string{"extract", "--input", input, "--output", t.TempDir()},
		&stdout,
		&stderr,
	)

	if exit != ExitPartial {
		t.Fatalf("exit=%d stderr=%q", exit, stderr.String())
	}
	if !strings.Contains(stderr.String(), "line 1") || !strings.Contains(stderr.String(), "skipping record") {
		t.Fatalf("stderr=%q", stderr.String())
	}
	if lines := bytes.Count(stdout.Bytes(), []byte("\n")); lines != 1 {
		t.Fatalf("stdout=%q; want one JSONL result", stdout.String())
	}
}

func TestExtractReportsActionableFailures(t *testing.T) {
	source := writeExtractEntry(t, []byte("\x89PNG\r\n\x1a\nimage"), "zstd")
	var stdout, stderr bytes.Buffer

	exit := Run(
		context.Background(),
		[]string{"extract", "--output", t.TempDir(), source},
		&stdout,
		&stderr,
	)

	if exit != ExitPartial {
		t.Fatalf("exit=%d stderr=%q", exit, stderr.String())
	}
	if !strings.Contains(stderr.String(), "unsupported content encoding") || !strings.Contains(stderr.String(), "gzip, deflate, and br") {
		t.Fatalf("stderr=%q; want supported-encoding guidance", stderr.String())
	}
}

func TestExtractRequiresOneInputModeAndOutput(t *testing.T) {
	tests := [][]string{
		{"extract"},
		{"extract", "entry"},
		{"extract", "--output", "out", "--input", "findings.jsonl", "entry"},
	}
	for _, args := range tests {
		var stdout, stderr bytes.Buffer
		if exit := Run(context.Background(), args, &stdout, &stderr); exit != ExitUsage {
			t.Errorf("%v exit=%d stderr=%q", args, exit, stderr.String())
		}
	}
}

func TestExtractRejectsZeroDecodedSizeLimit(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := Run(
		context.Background(),
		[]string{"extract", "--output", t.TempDir(), "--max-decoded-size", "0", "entry"},
		&stdout,
		&stderr,
	)
	if exit != ExitUsage || !strings.Contains(stderr.String(), "must be greater than zero") {
		t.Fatalf("exit=%d stderr=%q", exit, stderr.String())
	}
}

func writeExtractEntry(t *testing.T, body []byte, encoding string) string {
	t.Helper()
	key := "https://i.redd.it/cli.png"
	headers := "HTTP/1.1 200 OK\x00Content-Type: image/png\x00"
	if encoding != "" {
		headers += "Content-Encoding: " + encoding + "\x00"
	}
	headers += "\x00"
	stream0 := extractResponsePickle(headers)
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

func extractResponsePickle(headers string) []byte {
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

func gzipExtractBytes(data []byte) []byte {
	var out bytes.Buffer
	writer := gzip.NewWriter(&out)
	_, _ = writer.Write(data)
	_ = writer.Close()
	return out.Bytes()
}
