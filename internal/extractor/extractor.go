// Package extractor writes validated image bodies from parsed cache entries.
package extractor

import (
	"compress/gzip"
	"compress/zlib"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/dkujawski/dig-local-browser/internal/signatures"
	"github.com/dkujawski/dig-local-browser/internal/simplecache"
)

const DefaultMaxDecodedSize int64 = 256 << 20

var (
	ErrUnsupportedEncoding = errors.New("unsupported content encoding")
	ErrNotImage            = errors.New("decoded payload is not a recognized image")
	ErrDecodedTooLarge     = errors.New("decoded payload exceeds size limit")
	ErrNoBody              = errors.New("cache entry has no response body")
)

type Options struct {
	OutputDir      string
	MaxDecodedSize int64
}

type Result struct {
	SourcePath      string `json:"source_path"`
	URL             string `json:"url,omitempty"`
	MIMEType        string `json:"mime_type,omitempty"`
	ContentEncoding string `json:"content_encoding,omitempty"`
	RawSHA256       string `json:"raw_sha256"`
	DecodedSHA256   string `json:"decoded_sha256"`
	RawPath         string `json:"raw_path"`
	ImagePath       string `json:"image_path"`
	ImageType       string `json:"image_type"`
	Deduplicated    bool   `json:"deduplicated"`
}

// Extract parses path while it remains open, then atomically installs validated
// raw and decoded artifacts in options.OutputDir.
func Extract(path string, options Options) (Result, error) {
	var result Result
	if options.OutputDir == "" {
		return result, errors.New("output directory is required")
	}
	if options.MaxDecodedSize == 0 {
		options.MaxDecodedSize = DefaultMaxDecodedSize
	}
	if options.MaxDecodedSize < 0 {
		return result, errors.New("maximum decoded size must be greater than zero")
	}
	if err := os.MkdirAll(options.OutputDir, 0o700); err != nil {
		return result, fmt.Errorf("create output directory %q: %w", options.OutputDir, err)
	}

	source, err := os.Open(path)
	if err != nil {
		return result, fmt.Errorf("open cache entry %q: %w", path, err)
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return result, fmt.Errorf("stat cache entry %q: %w", path, err)
	}
	entry, err := simplecache.ParseFileReader(path, source, info.Size())
	if err != nil {
		return result, fmt.Errorf("parse cache entry %q: %w", path, err)
	}
	body, ok := bodyStream(entry)
	if !ok {
		return result, fmt.Errorf("%w: %q has no stream 1", ErrNoBody, path)
	}

	rawTemp, rawHash, err := copyRaw(options.OutputDir, source, body)
	if err != nil {
		return result, err
	}
	defer os.Remove(rawTemp)

	encodings, err := parseEncodings(entry.ContentEncoding)
	if err != nil {
		return result, err
	}
	decodedTemp := rawTemp
	decodedHash := rawHash
	if len(encodings) > 0 {
		decodedTemp, decodedHash, err = decodeRaw(rawTemp, options.OutputDir, encodings, options.MaxDecodedSize)
		if err != nil {
			return result, err
		}
		defer os.Remove(decodedTemp)
	}
	imageType, err := detectImage(decodedTemp)
	if err != nil {
		return result, err
	}

	imagePath := filepath.Join(options.OutputDir, decodedHash+"."+string(imageType))
	deduplicated, err := installTemp(decodedTemp, imagePath, decodedHash)
	if err != nil {
		return result, err
	}
	rawPath := imagePath
	if len(encodings) > 0 {
		rawPath = filepath.Join(options.OutputDir, rawHash+".raw")
		if _, err := installTemp(rawTemp, rawPath, rawHash); err != nil {
			return result, err
		}
	}
	return Result{
		SourcePath:      path,
		URL:             entry.URL,
		MIMEType:        entry.MIMEType,
		ContentEncoding: entry.ContentEncoding,
		RawSHA256:       rawHash,
		DecodedSHA256:   decodedHash,
		RawPath:         rawPath,
		ImagePath:       imagePath,
		ImageType:       string(imageType),
		Deduplicated:    deduplicated,
	}, nil
}

func bodyStream(entry *simplecache.Entry) (simplecache.Stream, bool) {
	for _, stream := range entry.Streams {
		if stream.Index == 1 {
			return stream, true
		}
	}
	return simplecache.Stream{}, false
}

func copyRaw(outputDir string, source io.ReaderAt, stream simplecache.Stream) (string, string, error) {
	temp, err := os.CreateTemp(outputDir, ".chromecarve-raw-*")
	if err != nil {
		return "", "", fmt.Errorf("create raw staging file in %q: %w", outputDir, err)
	}
	path := temp.Name()
	hash := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(temp, hash), io.NewSectionReader(source, stream.Offset, stream.Length))
	closeErr := temp.Close()
	if copyErr != nil {
		os.Remove(path)
		return "", "", fmt.Errorf("copy raw response body: %w", copyErr)
	}
	if closeErr != nil {
		os.Remove(path)
		return "", "", fmt.Errorf("close raw staging file: %w", closeErr)
	}
	return path, hex.EncodeToString(hash.Sum(nil)), nil
}

func parseEncodings(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var encodings []string
	for _, part := range strings.Split(value, ",") {
		encoding := strings.ToLower(strings.TrimSpace(part))
		if encoding == "" || encoding == "identity" {
			continue
		}
		switch encoding {
		case "gzip", "x-gzip", "deflate", "br":
			encodings = append(encodings, encoding)
		default:
			return nil, fmt.Errorf("%w %q; supported values are identity, gzip, deflate, and br", ErrUnsupportedEncoding, encoding)
		}
	}
	return encodings, nil
}

func decodeRaw(rawPath, outputDir string, encodings []string, limit int64) (string, string, error) {
	raw, err := os.Open(rawPath)
	if err != nil {
		return "", "", fmt.Errorf("open raw staging file: %w", err)
	}
	defer raw.Close()
	reader, closers, err := decoderChain(raw, encodings)
	if err != nil {
		return "", "", err
	}
	defer closeAll(closers)

	temp, err := os.CreateTemp(outputDir, ".chromecarve-decoded-*")
	if err != nil {
		return "", "", fmt.Errorf("create decoded staging file in %q: %w", outputDir, err)
	}
	path := temp.Name()
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(temp, hash), io.LimitReader(reader, limit+1))
	closeErr := temp.Close()
	if copyErr != nil {
		os.Remove(path)
		return "", "", fmt.Errorf("decode response body: %w", copyErr)
	}
	if closeErr != nil {
		os.Remove(path)
		return "", "", fmt.Errorf("close decoded staging file: %w", closeErr)
	}
	if written > limit {
		os.Remove(path)
		return "", "", fmt.Errorf("%w: decoded body exceeds %d bytes; raise --max-decoded-size if this image is trusted", ErrDecodedTooLarge, limit)
	}
	return path, hex.EncodeToString(hash.Sum(nil)), nil
}

func decoderChain(raw io.Reader, encodings []string) (io.Reader, []io.Closer, error) {
	reader := raw
	var closers []io.Closer
	for i := len(encodings) - 1; i >= 0; i-- {
		switch encodings[i] {
		case "gzip", "x-gzip":
			decoded, err := gzip.NewReader(reader)
			if err != nil {
				closeAll(closers)
				return nil, nil, fmt.Errorf("initialize gzip decoder: %w", err)
			}
			closers = append(closers, decoded)
			reader = decoded
		case "deflate":
			decoded, err := zlib.NewReader(reader)
			if err != nil {
				closeAll(closers)
				return nil, nil, fmt.Errorf("initialize deflate decoder: %w", err)
			}
			closers = append(closers, decoded)
			reader = decoded
		case "br":
			reader = brotli.NewReader(reader)
		}
	}
	return reader, closers, nil
}

func closeAll(closers []io.Closer) {
	for i := len(closers) - 1; i >= 0; i-- {
		_ = closers[i].Close()
	}
}

func detectImage(path string) (signatures.Type, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open decoded staging file: %w", err)
	}
	defer file.Close()
	header := make([]byte, 64)
	n, err := file.Read(header)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read decoded image header: %w", err)
	}
	for _, match := range signatures.Find(header[:n]) {
		if match.Offset == 0 {
			return match.Type, nil
		}
	}
	return "", fmt.Errorf("%w; verify the cache entry and Content-Encoding metadata", ErrNotImage)
}

func installTemp(tempPath, targetPath, expectedHash string) (bool, error) {
	if _, err := os.Stat(targetPath); err == nil {
		actualHash, hashErr := hashFile(targetPath)
		if hashErr != nil {
			return false, hashErr
		}
		if actualHash != expectedHash {
			return false, fmt.Errorf("refuse to replace existing artifact %q: content does not match its digest name", targetPath)
		}
		return true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("inspect destination artifact %q: %w", targetPath, err)
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		return false, fmt.Errorf("install artifact %q: %w", targetPath, err)
	}
	return false, nil
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open existing artifact %q: %w", path, err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash existing artifact %q: %w", path, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
