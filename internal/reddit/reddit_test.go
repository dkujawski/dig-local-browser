package reddit

import "testing"

func TestNormalizeEscaped(t *testing.T) {
	for _, input := range []string{`https:\/\/i.redd.it\/cat.jpg`, `https:\u002F\u002Fpreview.redd.it\/cat.png`, `i\.redd\.it`} {
		if got := NormalizeEscaped(input); got == input {
			t.Errorf("NormalizeEscaped(%q) did not normalize", input)
		}
	}
}

func TestFindSignals(t *testing.T) {
	matches := FindSignals([]byte("GET https:\\/\\/preview.redd.it\\/x Content-Type: image/jpeg"))
	if !matches.Reddit || !matches.ImageContentType {
		t.Fatalf("FindSignals() = %+v", matches)
	}
}
