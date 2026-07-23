package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"

	"github.com/dkujawski/dig-local-browser/internal/simplecache"
)

func runInspect(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("inspect", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { fmt.Fprintln(stderr, "Usage: chromecarve inspect PATH") }
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return ExitSuccess
		}
		return ExitUsage
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(stderr, "inspect requires exactly one cache-entry path")
		flags.Usage()
		return ExitUsage
	}
	entry, err := simplecache.ParseFile(flags.Arg(0))
	if entry != nil {
		writeInspection(stdout, entry)
	}
	if err == nil {
		return ExitSuccess
	}
	fmt.Fprintf(stderr, "inspect %q: %v\n", flags.Arg(0), err)
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
