package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dkujawski/dig-local-browser/internal/extractor"
	"github.com/dkujawski/dig-local-browser/internal/simplecache"
)

func runExtract(args []string, stdout, stderr io.Writer) int {
	leadingPath, flagArgs := extractLeadingPath(args)
	flags := flag.NewFlagSet("extract", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var input, output string
	var maxDecoded sizeFlag = sizeFlag(extractor.DefaultMaxDecodedSize)
	flags.StringVar(&input, "input", "", "scan JSONL file containing candidate paths")
	flags.StringVar(&output, "output", "", "directory for extracted artifacts")
	flags.Var(&maxDecoded, "max-decoded-size", "maximum bytes per decoded image (supports KiB/MiB/GiB)")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: chromecarve extract --output DIR PATH")
		fmt.Fprintln(stderr, "   or: chromecarve extract --input FILE --output DIR")
		flags.PrintDefaults()
	}
	if err := flags.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return ExitSuccess
		}
		return ExitUsage
	}
	paths := flags.Args()
	if leadingPath != "" {
		paths = append([]string{leadingPath}, paths...)
	}
	if output == "" {
		fmt.Fprintln(stderr, "extract requires --output DIR")
		flags.Usage()
		return ExitUsage
	}
	if (input == "" && len(paths) != 1) || (input != "" && len(paths) != 0) {
		fmt.Fprintln(stderr, "extract requires either one cache-entry path or --input FILE")
		flags.Usage()
		return ExitUsage
	}
	options := extractor.Options{OutputDir: output, MaxDecodedSize: int64(maxDecoded)}
	if input != "" {
		return extractJSONL(input, options, stdout, stderr)
	}
	return extractPath(paths[0], options, stdout, stderr)
}

func extractLeadingPath(args []string) (string, []string) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return args[0], args[1:]
	}
	return "", args
}

func extractJSONL(path string, options extractor.Options, stdout, stderr io.Writer) int {
	file, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(stderr, "extract input %q: %v; provide a readable JSONL file created by chromecarve scan\n", path, err)
		return ExitFatal
	}
	defer file.Close()

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
			fmt.Fprintf(stderr, "extract input %q line %d: invalid JSON: %v; skipping record\n", path, lineNumber, err)
			hadErrors = true
			continue
		}
		if record.Path == "" {
			fmt.Fprintf(stderr, "extract input %q line %d: missing non-empty \"path\" field; skipping record\n", path, lineNumber)
			hadErrors = true
			continue
		}
		if extractPath(record.Path, options, stdout, stderr) != ExitSuccess {
			hadErrors = true
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(stderr, "extract input %q after line %d: %v; JSONL records must be smaller than %d bytes\n", path, lineNumber, err, maxJSONLRecordSize)
		return ExitFatal
	}
	if hadErrors {
		return ExitPartial
	}
	return ExitSuccess
}

func extractPath(path string, options extractor.Options, stdout, stderr io.Writer) int {
	result, err := extractor.Extract(path, options)
	if err == nil {
		if encodeErr := json.NewEncoder(stdout).Encode(result); encodeErr != nil {
			fmt.Fprintf(stderr, "extract %q: write JSON result: %v\n", path, encodeErr)
			return ExitFatal
		}
		return ExitSuccess
	}
	fmt.Fprintf(stderr, "extract %q: %v\n", path, err)
	switch {
	case errors.Is(err, simplecache.ErrNotSimpleCache), errors.Is(err, simplecache.ErrUnsupportedVersion):
		return ExitUnsupported
	case errors.Is(err, simplecache.ErrCorruptEntry),
		errors.Is(err, simplecache.ErrTruncatedEntry),
		errors.Is(err, extractor.ErrUnsupportedEncoding),
		errors.Is(err, extractor.ErrNotImage),
		errors.Is(err, extractor.ErrDecodedTooLarge),
		errors.Is(err, extractor.ErrNoBody):
		return ExitPartial
	default:
		return ExitFatal
	}
}
