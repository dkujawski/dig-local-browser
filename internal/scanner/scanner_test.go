package scanner

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/dkujawski/dig-local-browser/internal/simplecache"
)

func TestScanWritesCandidate(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Cache", "Cache_Data", "abcdef_0")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	data := append([]byte{0x30, 0x5c, 0x72, 0xa7, 0x1b, 0x6d, 0xfb, 0xfc}, []byte(" https://i.redd.it/a.jpg Content-Type: image/jpeg \xff\xd8\xff")...)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	stats, err := Scan(context.Background(), Config{Roots: []string{root}, Workers: 2, MaxContentScan: 1024, IncludeHidden: true}, &out, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Candidates != 1 || out.Len() == 0 {
		t.Fatalf("stats=%+v output=%q", stats, out.String())
	}
}

func TestScanHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Scan(ctx, Config{Roots: []string{t.TempDir()}, Workers: 1}, &bytes.Buffer{}, nil)
	if err == nil {
		t.Fatal("Scan() error = nil; want cancellation")
	}
}

func TestScanBoundsContentInspection(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "ordinary.bin")
	if err := os.WriteFile(path, append(make([]byte, 16), []byte("https://i.redd.it/late.jpg")...), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	stats, err := Scan(context.Background(), Config{Roots: []string{root}, Workers: 1, MaxContentScan: 8, IncludeHidden: true}, &out, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Candidates != 0 || out.Len() != 0 {
		t.Fatalf("scanner read beyond limit: stats=%+v output=%q", stats, out.String())
	}
}

func TestScanFollowsDirectorySymlinkWhenEnabled(t *testing.T) {
	target := t.TempDir()
	path := filepath.Join(target, "Cache_Data", "abcdef_0")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("https://i.redd.it/image.jpg"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Symlink(target, filepath.Join(root, "linked-cache")); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	stats, err := Scan(context.Background(), Config{Roots: []string{root}, Workers: 1, MaxContentScan: 1024, IncludeHidden: true, FollowSymlinks: true}, &out, nil)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Candidates != 1 {
		t.Fatalf("stats=%+v output=%q; want linked candidate", stats, out.String())
	}
}

func TestScanScoresParsedCacheURL(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Cache_Data", "0123456789abcdef_0")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	key := "https://i.redd.it/image.jpg"
	data := make([]byte, int(simplecache.FileHeaderSize)+len(key)+int(simplecache.FileEOFSize))
	binary.LittleEndian.PutUint64(data[0:8], simplecache.SimpleCacheMagic)
	binary.LittleEndian.PutUint32(data[8:12], simplecache.EntryVersion)
	binary.LittleEndian.PutUint32(data[12:16], uint32(len(key)))
	copy(data[int(simplecache.FileHeaderSize):], key)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	_, err := Scan(context.Background(), Config{Roots: []string{root}, Workers: 1, MaxContentScan: 1024, IncludeHidden: true}, &out, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"valid-cache-key"`)) {
		t.Fatalf("inventory did not score parsed URL: %s", out.String())
	}
}
