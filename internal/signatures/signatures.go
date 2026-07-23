// Package signatures locates recognized image container signatures in bounded data.
package signatures

import (
	"bytes"
	"encoding/binary"
)

type Type string

const (
	JPEG Type = "jpeg"
	PNG  Type = "png"
	GIF  Type = "gif"
	WebP Type = "webp"
	AVIF Type = "avif"
	HEIC Type = "heic"
	HEIF Type = "heif"
)

type Match struct {
	Type   Type  `json:"type"`
	Offset int64 `json:"offset"`
}

// Detect identifies an image container beginning at data[0]. totalSize is the
// complete payload length, allowing bounded header reads for length-bearing
// formats such as WebP.
func Detect(data []byte, totalSize int64) (Type, bool) {
	switch {
	case len(data) >= 3 && bytes.Equal(data[:3], []byte{0xff, 0xd8, 0xff}):
		return JPEG, true
	case len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}):
		return PNG, true
	case len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a"):
		return GIF, true
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		size := int64(binary.LittleEndian.Uint32(data[4:8])) + 8
		return WebP, size >= 12 && size <= totalSize
	case len(data) >= 12 && string(data[4:8]) == "ftyp":
		switch string(data[8:12]) {
		case "avif", "avis":
			return AVIF, true
		case "heic", "heix":
			return HEIC, true
		case "mif1":
			return HEIF, true
		}
	}
	return "", false
}

// Find returns every plausible signature in data. It does not infer full image bounds.
func Find(data []byte) []Match {
	var out []Match
	out = appendPattern(out, data, JPEG, []byte{0xff, 0xd8, 0xff})
	out = appendPattern(out, data, PNG, []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a})
	out = appendPattern(out, data, GIF, []byte("GIF87a"))
	out = appendPattern(out, data, GIF, []byte("GIF89a"))
	for at := 0; at+12 <= len(data); at++ {
		if string(data[at:at+4]) == "RIFF" && string(data[at+8:at+12]) == "WEBP" {
			sz := uint64(binary.LittleEndian.Uint32(data[at+4:at+8])) + 8
			if sz >= 12 && sz <= uint64(len(data)-at) {
				out = append(out, Match{Type: WebP, Offset: int64(at)})
			}
		}
		if at+12 <= len(data) && string(data[at+4:at+8]) == "ftyp" {
			switch string(data[at+8 : at+12]) {
			case "avif", "avis":
				out = append(out, Match{Type: AVIF, Offset: int64(at)})
			case "heic", "heix":
				out = append(out, Match{Type: HEIC, Offset: int64(at)})
			case "mif1":
				out = append(out, Match{Type: HEIF, Offset: int64(at)})
			}
		}
	}
	return out
}

func appendPattern(out []Match, data []byte, typ Type, pattern []byte) []Match {
	for from := 0; from < len(data); {
		at := bytes.Index(data[from:], pattern)
		if at < 0 {
			break
		}
		out = append(out, Match{Type: typ, Offset: int64(from + at)})
		from += at + 1
	}
	return out
}
