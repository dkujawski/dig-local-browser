package simplecache

import "io"

const (
	EntryVersion   uint32 = 5
	FileHeaderSize int64  = 24
	FileEOFSize    int64  = 24
	FinalMagic     uint64 = 0xf4fa6f45970d41d8

	flagHasCRC32     uint32 = 1 << 0
	flagHasKeySHA256 uint32 = 1 << 1
)

type EntryHeader struct {
	InitialMagic uint64 `json:"initial_magic"`
	Version      uint32 `json:"version"`
	KeyLength    uint32 `json:"key_length"`
	KeyHash      uint32 `json:"key_hash"`
}

type Stream struct {
	Index         int    `json:"index"`
	Offset        int64  `json:"offset"`
	Length        int64  `json:"length"`
	EOFOffset     int64  `json:"eof_offset"`
	HasCRC32      bool   `json:"has_crc32"`
	ExpectedCRC32 uint32 `json:"expected_crc32,omitempty"`
	CRCVerified   bool   `json:"crc_verified"`
}

type Entry struct {
	Path              string              `json:"path,omitempty"`
	Key               string              `json:"key"`
	URL               string              `json:"url,omitempty"`
	Header            EntryHeader         `json:"header"`
	Streams           []Stream            `json:"streams"`
	HTTPStatus        int                 `json:"http_status,omitempty"`
	HTTPStatusLine    string              `json:"http_status_line,omitempty"`
	HTTPHeaders       map[string][]string `json:"http_headers,omitempty"`
	MIMEType          string              `json:"mime_type,omitempty"`
	ContentEncoding   string              `json:"content_encoding,omitempty"`
	BodySHA256        string              `json:"body_sha256,omitempty"`
	KeyHashVerified   bool                `json:"key_hash_verified"`
	KeySHA256Verified *bool               `json:"key_sha256_verified,omitempty"`
	Body              io.Reader           `json:"-"`
	Warnings          []string            `json:"warnings"`
}
