// Package scoring centralizes the transparent candidate scoring policy.
package scoring

type Features struct {
	SimpleCache, CacheStructure, RedditURL, ImageContentType bool
	ValidCacheKey, ImageSignature, ChromePath, ServiceWorker bool
	InTimeWindow, EntryFilename, Unrelated, InvalidOffsets   bool
}

type Result struct {
	Confidence int
	Signals    []string
}

type rule struct {
	enabled bool
	points  int
	signal  string
}

func Evaluate(f Features) Result {
	rules := []rule{
		{f.SimpleCache, 40, "simple-cache-magic"},
		{f.CacheStructure, 25, "cache-data-directory"},
		{f.RedditURL, 20, "reddit-url"},
		{f.ImageContentType, 15, "image-content-type"},
		{f.ValidCacheKey, 15, "valid-cache-key"},
		{f.ImageSignature, 10, "image-signature"},
		{f.ChromePath, 10, "chrome-path"},
		{f.ServiceWorker, 8, "service-worker-cache"},
		{f.InTimeWindow, 5, "modification-time-window"},
		{f.EntryFilename, 5, "cache-entry-filename"},
		{f.Unrelated, -10, "unrelated-application-data"},
		{f.InvalidOffsets, -20, "invalid-offsets"},
	}
	var out Result
	for _, r := range rules {
		if r.enabled {
			out.Confidence += r.points
			out.Signals = append(out.Signals, r.signal)
		}
	}
	return out
}
