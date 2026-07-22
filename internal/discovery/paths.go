// Package discovery identifies Chromium-like layout signals without treating paths as proof.
package discovery

import (
	"path/filepath"
	"regexp"
	"strings"
)

var entryName = regexp.MustCompile(`(?i)^[0-9a-f]+_(?:[0-2]|s)$`)

type PathSignals struct {
	CacheStructure bool
	ChromePath     bool
	ServiceWorker  bool
	EntryFilename  bool
	Unrelated      bool
}

func ClassifyPath(path string) PathSignals {
	clean := strings.ToLower(filepath.ToSlash(path))
	base := filepath.Base(path)
	cacheLayouts := []string{"/cache/cache_data/", "/cache_data/", "/service worker/cachestorage/", "/service worker/database/", "/indexeddb/", "/local storage/", "/session storage/", "/blob_storage/", "/code cache/", "/gpucache/"}
	var out PathSignals
	for _, layout := range cacheLayouts {
		if strings.Contains(clean, layout) {
			out.CacheStructure = true
			break
		}
	}
	out.ChromePath = strings.Contains(clean, "/google/chrome/") || strings.Contains(clean, "/chromium/")
	out.ServiceWorker = strings.Contains(clean, "/service worker/cachestorage/")
	out.EntryFilename = entryName.MatchString(base)
	out.Unrelated = strings.Contains(clean, "/node_modules/") || strings.Contains(clean, "/pkg/mod/") || strings.HasSuffix(clean, ".go") || strings.HasSuffix(clean, ".js")
	return out
}
