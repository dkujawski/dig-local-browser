package signatures

import "testing"

func TestFindRecognizedImages(t *testing.T) {
	webp := append([]byte("RIFF\x0c\x00\x00\x00WEBP"), make([]byte, 12)...)
	data := append([]byte("prefix"), []byte("\xff\xd8\xffmiddle\x89PNG\r\n\x1a\n")...)
	data = append(data, webp...)
	data = append(data, []byte("\x00\x00\x00\x18ftypavif")...)
	found := Find(data)
	want := map[Type]bool{JPEG: true, PNG: true, WebP: true, AVIF: true}
	for _, sig := range found {
		delete(want, sig.Type)
	}
	if len(want) != 0 {
		t.Fatalf("missing signatures: %v; got %v", want, found)
	}
}

func TestRejectsInvalidWebP(t *testing.T) {
	if got := Find([]byte("RIFF\xff\xff\xff\xffWEBP")); len(got) != 0 {
		t.Fatalf("Find() = %v; want no signatures", got)
	}
}
