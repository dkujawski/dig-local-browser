package cli

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dkujawski/dig-local-browser/internal/simplecache"
)

func TestInspectDisplaysParsedEntry(t *testing.T) {
	key := "https://example.test/metadata"
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

func TestCommandHelpExitsSuccessfully(t *testing.T) {
	for _, command := range []string{"scan", "inspect"} {
		var stdout, stderr bytes.Buffer
		if exit := Run(context.Background(), []string{command, "--help"}, &stdout, &stderr); exit != ExitSuccess {
			t.Errorf("%s --help exit = %d, want %d", command, exit, ExitSuccess)
		}
	}
}
