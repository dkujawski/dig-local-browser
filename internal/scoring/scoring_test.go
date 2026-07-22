package scoring

import "testing"

func TestEvaluateKeepsExplainableSignals(t *testing.T) {
	got := Evaluate(Features{SimpleCache: true, CacheStructure: true, RedditURL: true, ImageContentType: true, ImageSignature: true, ChromePath: true, InTimeWindow: true, EntryFilename: true})
	if got.Confidence != 130 {
		t.Fatalf("confidence = %d; want 130", got.Confidence)
	}
	if len(got.Signals) != 8 {
		t.Fatalf("signals = %v", got.Signals)
	}
}
