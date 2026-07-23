package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/dkujawski/dig-local-browser/internal/simplecache"
)

const maxJSONLRecordSize = 16 << 20

func runInspect(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("inspect", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var input string
	flags.StringVar(&input, "input", "", "scan JSONL file containing candidate paths")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: chromecarve inspect PATH")
		fmt.Fprintln(stderr, "   or: chromecarve inspect --input FILE")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return ExitSuccess
		}
		return ExitUsage
	}
	if (input == "" && flags.NArg() != 1) || (input != "" && flags.NArg() != 0) {
		fmt.Fprintln(stderr, "inspect requires either one cache-entry path or --input FILE")
		flags.Usage()
		return ExitUsage
	}
	if input != "" {
		return inspectJSONL(input, stdout, stderr)
	}
	return inspectPath(flags.Arg(0), stdout, stderr)
}

func inspectJSONL(path string, stdout, stderr io.Writer) int {
	file, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(stderr, "inspect input %q: %v; provide a readable JSONL file created by chromecarve scan\n", path, err)
		return ExitFatal
	}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), maxJSONLRecordSize)
	lineNumber := 0
	hadErrors := false
	for scanner.Scan() {
		lineNumber++
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		var record struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			fmt.Fprintf(stderr, "inspect input %q line %d: invalid JSON: %v; skipping record\n", path, lineNumber, err)
			hadErrors = true
			continue
		}
		if record.Path == "" {
			fmt.Fprintf(stderr, "inspect input %q line %d: missing non-empty \"path\" field; skipping record\n", path, lineNumber)
			hadErrors = true
			continue
		}
		if inspectPath(record.Path, stdout, stderr) != ExitSuccess {
			hadErrors = true
		}
	}
	if err := scanner.Err(); err != nil {
		_ = file.Close()
		fmt.Fprintf(stderr, "inspect input %q after line %d: %v; JSONL records must be smaller than %d bytes\n", path, lineNumber, err, maxJSONLRecordSize)
		return ExitFatal
	}
	if err := file.Close(); err != nil {
		fmt.Fprintf(stderr, "inspect input %q: close failed: %v\n", path, err)
		return ExitFatal
	}
	if hadErrors {
		return ExitPartial
	}
	return ExitSuccess
}

func inspectPath(path string, stdout, stderr io.Writer) int {
	entry, err := simplecache.ParseFile(path)
	if entry != nil {
		writeInspection(stdout, entry)
	}
	if err == nil {
		return ExitSuccess
	}
	fmt.Fprintf(stderr, "inspect %q: %v\n", path, err)
	if errors.Is(err, simplecache.ErrNotSimpleCache) || errors.Is(err, simplecache.ErrUnsupportedVersion) {
		return ExitUnsupported
	}
	if errors.Is(err, simplecache.ErrCorruptEntry) || errors.Is(err, simplecache.ErrTruncatedEntry) {
		return ExitPartial
	}
	return ExitFatal
}

func writeInspection(w io.Writer, entry *simplecache.Entry) {
	fmt.Fprintln(w, "Format: Chromium Simple Cache")
	fmt.Fprintf(w, "Path: %s\n", entry.Path)
	fmt.Fprintf(w, "Version: %d\n", entry.Header.Version)
	fmt.Fprintf(w, "Key length: %d\n", entry.Header.KeyLength)
	fmt.Fprintf(w, "Key hash: %08x (verified=%t)\n", entry.Header.KeyHash, entry.KeyHashVerified)
	fmt.Fprintf(w, "Key: %s\n", entry.Key)
	if entry.URL != "" {
		fmt.Fprintf(w, "URL: %s\n", entry.URL)
	}
	for _, stream := range entry.Streams {
		fmt.Fprintf(w, "Stream: stream %d offset=%d length=%d eof_offset=%d crc32=%t verified=%t\n", stream.Index, stream.Offset, stream.Length, stream.EOFOffset, stream.HasCRC32, stream.CRCVerified)
	}
	if entry.HTTPStatusLine != "" {
		fmt.Fprintf(w, "HTTP status: %s\n", entry.HTTPStatusLine)
	}
	if entry.MIMEType != "" {
		fmt.Fprintf(w, "MIME type: %s\n", entry.MIMEType)
	}
	if entry.ContentEncoding != "" {
		fmt.Fprintf(w, "Content encoding: %s\n", entry.ContentEncoding)
	}
	if entry.BodySHA256 != "" {
		fmt.Fprintf(w, "Body SHA-256: %s\n", entry.BodySHA256)
	}
	if len(entry.HTTPHeaders) > 0 {
		fmt.Fprintln(w, "HTTP headers:")
		names := make([]string, 0, len(entry.HTTPHeaders))
		for name := range entry.HTTPHeaders {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			for _, value := range entry.HTTPHeaders[name] {
				fmt.Fprintf(w, "  %s: %s\n", name, value)
			}
		}
	}
	for _, warning := range entry.Warnings {
		fmt.Fprintf(w, "Warning: %s\n", warning)
	}
}
