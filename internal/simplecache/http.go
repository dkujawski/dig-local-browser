package simplecache

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/textproto"
	"strconv"
	"strings"
)

const (
	responseInfoVersionMask              = uint32(0xff)
	responseInfoHasExtraFlags            = uint32(1 << 31)
	responseExtraHasOriginalResponseTime = uint32(1 << 2)
)

type responseMetadata struct {
	statusLine string
	status     int
	headers    map[string][]string
}

func parseResponseMetadata(data []byte) (responseMetadata, error) {
	var out responseMetadata
	if len(data) < 4 {
		return out, errors.New("HTTP metadata is shorter than a Pickle header")
	}
	payloadSize := int64(binary.LittleEndian.Uint32(data[:4]))
	headerSize := int64(len(data)) - payloadSize
	if headerSize < 4 || headerSize%4 != 0 || payloadSize < 0 || headerSize > int64(len(data)) {
		return out, errors.New("invalid Chromium Pickle payload size")
	}
	cursor := int(headerSize)
	readU32 := func() (uint32, error) {
		if cursor > len(data)-4 {
			return 0, ioBoundsError("uint32", cursor, len(data))
		}
		value := binary.LittleEndian.Uint32(data[cursor : cursor+4])
		cursor += 4
		return value, nil
	}
	readI64 := func() error {
		if cursor > len(data)-8 {
			return ioBoundsError("int64", cursor, len(data))
		}
		cursor += 8
		return nil
	}
	flags, err := readU32()
	if err != nil {
		return out, err
	}
	version := flags & responseInfoVersionMask
	if version < 1 || version > 3 {
		return out, fmt.Errorf("unsupported HTTP response-info version %d", version)
	}
	var extraFlags uint32
	if flags&responseInfoHasExtraFlags != 0 {
		extraFlags, err = readU32()
		if err != nil {
			return out, err
		}
	}
	if err := readI64(); err != nil {
		return out, err
	}
	if err := readI64(); err != nil {
		return out, err
	}
	if extraFlags&responseExtraHasOriginalResponseTime != 0 {
		if err := readI64(); err != nil {
			return out, err
		}
	}
	length, err := readU32()
	if err != nil {
		return out, err
	}
	if uint64(length) > uint64(len(data)-cursor) {
		return out, errors.New("HTTP header string exceeds Pickle payload")
	}
	raw := string(data[cursor : cursor+int(length)])
	return parseRawHeaders(raw)
}

func parseRawHeaders(raw string) (responseMetadata, error) {
	parts := strings.Split(raw, "\x00")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return responseMetadata{}, errors.New("HTTP response has no status line")
	}
	out := responseMetadata{statusLine: strings.TrimSpace(parts[0]), headers: make(map[string][]string)}
	statusFields := strings.Fields(out.statusLine)
	if len(statusFields) < 2 {
		return responseMetadata{}, errors.New("malformed HTTP status line")
	}
	status, err := strconv.Atoi(statusFields[1])
	if err != nil || status < 100 || status > 999 {
		return responseMetadata{}, errors.New("malformed HTTP status code")
	}
	out.status = status
	for _, line := range parts[1:] {
		if line == "" {
			continue
		}
		name, value, found := strings.Cut(line, ":")
		name = textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(name))
		if !found || name == "" {
			continue
		}
		out.headers[name] = append(out.headers[name], strings.TrimSpace(value))
	}
	return out, nil
}

func ioBoundsError(kind string, offset, size int) error {
	return fmt.Errorf("truncated Pickle %s at offset %d of %d", kind, offset, size)
}
