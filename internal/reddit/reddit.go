// Package reddit recognizes Reddit resources without making network requests.
package reddit

import (
	"regexp"
	"strings"
)

var unicodeSlash = regexp.MustCompile(`(?i)\\u0*02f`)

type Signals struct {
	Reddit           bool
	ImageContentType bool
	Matched          []string
}

// NormalizeEscaped normalizes common JSON, JavaScript, and regex URL escaping.
func NormalizeEscaped(s string) string {
	s = unicodeSlash.ReplaceAllString(s, "/")
	s = strings.ReplaceAll(s, `\/`, "/")
	s = strings.ReplaceAll(s, `\.`, ".")
	return s
}

func FindSignals(data []byte) Signals {
	normalized := strings.ToLower(NormalizeEscaped(string(data)))
	var out Signals
	for _, needle := range []string{"i.redd.it", "preview.redd.it", "external-preview.redd.it", "redditmedia.com", "redditstatic.com", "reddit.com/media"} {
		if strings.Contains(normalized, needle) {
			out.Reddit = true
			out.Matched = append(out.Matched, needle)
		}
	}
	if strings.Contains(normalized, "content-type: image/") {
		out.ImageContentType = true
		out.Matched = append(out.Matched, "Content-Type: image/")
	}
	return out
}
