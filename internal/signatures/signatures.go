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
