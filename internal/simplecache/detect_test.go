package simplecache

import (
	"bytes"
	"errors"
	"testing"
)

func TestDetectMagicLittleEndian(t *testing.T) {
	data := append([]byte{0x30, 0x5c, 0x72, 0xa7, 0x1b, 0x6d, 0xfb, 0xfc}, make([]byte, 16)...)
	ok, err := Detect(bytes.NewReader(data), int64(len(data)))
	if err != nil || !ok {
		t.Fatalf("Detect() = %v, %v; want true, nil", ok, err)
	}
}

func TestDetectRejectsShortAndWrongInput(t *testing.T) {
	for _, data := range [][]byte{{1, 2, 3}, make([]byte, 8)} {
		ok, err := Detect(bytes.NewReader(data), int64(len(data)))
		if ok {
			t.Fatalf("Detect(%x) unexpectedly succeeded", data)
		}
		if len(data) < 8 && !errors.Is(err, ErrTruncatedEntry) {
			t.Fatalf("error = %v; want ErrTruncatedEntry", err)
		}
	}
}
