package cli

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dkujawski/dig-local-browser/internal/manifest"
	"github.com/dkujawski/dig-local-browser/internal/simplecache"
)

func TestInspectDisplaysParsedEntry(t *testing.T) {
	key := "https://example.test/metadata"
	path := writeSimpleCacheEntry(t, key)

	var stdout, stderr bytes.Buffer
	exit := Run(context.Background(), []string{"inspect", path}, &stdout, &stderr)
	if exit != ExitSuccess {
		t.Fatalf("exit=%d stderr=%q", exit, stderr.String())
	}
	for _, want := range []string{"Chromium Simple Cache", key, "stream 2", "offset="} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("output %q missing %q", stdout.String(), want)
		}
	}
}

func TestInspectReadsCandidatePathsFromJSONLInput(t *testing.T) {
	firstKey := "https://example.test/first"
	secondKey := "https://example.test/second"
	input := writeCandidateJSONL(t,
		manifest.Candidate{Path: writeSimpleCacheEntry(t, firstKey)},
		manifest.Candidate{Path: writeSimpleCacheEntry(t, secondKey)},
	)

	var stdout, stderr bytes.Buffer
	exit := Run(context.Background(), []string{"inspect", "--input", input}, &stdout, &stderr)

	if exit != ExitSuccess {
		t.Fatalf("exit=%d stderr=%q", exit, stderr.String())
	}
	for _, want := range []string{firstKey, secondKey} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("output %q missing %q", stdout.String(), want)
		}
	}
}

func TestInspectJSONLContinuesAfterInvalidRecord(t *testing.T) {
	key := "https://example.test/valid"
	input := filepath.Join(t.TempDir(), "findings.jsonl")
	valid, err := json.Marshal(manifest.Candidate{Path: writeSimpleCacheEntry(t, key)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input, append([]byte("{not json}\n"), append(valid, '\n')...), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	exit := Run(context.Background(), []string{"inspect", "--input", input}, &stdout, &stderr)

	if exit != ExitPartial {
		t.Fatalf("exit=%d, want %d; stderr=%q", exit, ExitPartial, stderr.String())
	}
	if !strings.Contains(stderr.String(), "line 1") || !strings.Contains(stderr.String(), "invalid JSON") {
		t.Errorf("stderr=%q; want actionable line-specific JSON error", stderr.String())
	}
	if !strings.Contains(stdout.String(), key) {
		t.Errorf("output %q missing later valid record %q", stdout.String(), key)
	}
}

func TestInspectRejectsPathAndJSONLInputTogether(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := Run(context.Background(), []string{"inspect", "--input", "findings.jsonl", "entry"}, &stdout, &stderr)

	if exit != ExitUsage {
		t.Fatalf("exit=%d, want %d", exit, ExitUsage)
	}
	if !strings.Contains(stderr.String(), "either one cache-entry path or --input") {
		t.Errorf("stderr=%q; want input guidance", stderr.String())
	}
}

func writeSimpleCacheEntry(t *testing.T, key string) string {
	t.Helper()
	payload := []byte("auxiliary")
	data := make([]byte, int(simplecache.FileHeaderSize)+len(key)+len(payload)+int(simplecache.FileEOFSize))
	binary.LittleEndian.PutUint64(data[0:8], simplecache.SimpleCacheMagic)
	binary.LittleEndian.PutUint32(data[8:12], simplecache.EntryVersion)
	binary.LittleEndian.PutUint32(data[12:16], uint32(len(key)))
	copy(data[int(simplecache.FileHeaderSize):], key)
	copy(data[int(simplecache.FileHeaderSize)+len(key):], payload)
	footer := len(data) - int(simplecache.FileEOFSize)
	binary.LittleEndian.PutUint64(data[footer:footer+8], simplecache.FinalMagic)
	path := filepath.Join(t.TempDir(), "0123456789abcdef_1")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeCandidateJSONL(t *testing.T, candidates ...manifest.Candidate) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "findings.jsonl")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(file)
	for _, candidate := range candidates {
		if err := encoder.Encode(candidate); err != nil {
			t.Fatal(err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCommandHelpExitsSuccessfully(t *testing.T) {
	for _, command := range []string{"scan", "inspect"} {
		var stdout, stderr bytes.Buffer
		if exit := Run(context.Background(), []string{command, "--help"}, &stdout, &stderr); exit != ExitSuccess {
			t.Errorf("%s --help exit = %d, want %d", command, exit, ExitSuccess)
		}
	}
}
