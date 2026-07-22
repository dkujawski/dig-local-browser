package manifest

import (
	"bytes"
	"strings"
	"testing"
)

func TestJSONLWriter(t *testing.T) {
	var out bytes.Buffer
	w := NewWriter(&out)
	if err := w.Write(Candidate{Path: "/tmp/a", Confidence: 40, Signals: []string{"simple-cache-magic"}}); err != nil {
		t.Fatal(err)
	}
	if lines := strings.Count(out.String(), "\n"); lines != 1 {
		t.Fatalf("newlines = %d; want 1", lines)
	}
	if !strings.Contains(out.String(), `"path":"/tmp/a"`) {
		t.Fatalf("unexpected JSONL: %s", out.String())
	}
}
