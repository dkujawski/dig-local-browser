package discovery

import "testing"

func TestClassifyCacheDataPath(t *testing.T) {
	got := ClassifyPath("/Users/me/Library/Caches/Google/Chrome/Default/Cache/Cache_Data/a1b2_0")
	if !got.CacheStructure || !got.ChromePath || !got.EntryFilename {
		t.Fatalf("ClassifyPath() = %+v", got)
	}
}

func TestClassifyUnrelatedPath(t *testing.T) {
	got := ClassifyPath("/tmp/project/main.go")
	if got.CacheStructure || got.EntryFilename {
		t.Fatalf("ClassifyPath() = %+v", got)
	}
}
