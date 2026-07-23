// Package cli provides the chromecarve command-line interface.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/dkujawski/dig-local-browser/internal/scanner"
)

const (
	ExitSuccess     = 0
	ExitPartial     = 1
	ExitUsage       = 2
	ExitFatal       = 3
	ExitUnsupported = 4
)

type stringsFlag []string

func (s *stringsFlag) String() string     { return strings.Join(*s, ",") }
func (s *stringsFlag) Set(v string) error { *s = append(*s, v); return nil }

type sizeFlag int64

func (s *sizeFlag) String() string { return strconv.FormatInt(int64(*s), 10) }
func (s *sizeFlag) Set(v string) error {
	multiplier := int64(1)
	upper := strings.ToUpper(strings.TrimSpace(v))
	for suffix, factor := range map[string]int64{"KIB": 1 << 10, "MIB": 1 << 20, "GIB": 1 << 30, "KB": 1000, "MB": 1000 * 1000, "GB": 1000 * 1000 * 1000} {
		if strings.HasSuffix(upper, suffix) {
			multiplier = factor
			upper = strings.TrimSpace(strings.TrimSuffix(upper, suffix))
			break
		}
	}
	n, err := strconv.ParseInt(upper, 10, 64)
	if err != nil || n < 0 {
		return fmt.Errorf("invalid size %q", v)
	}
	*s = sizeFlag(n * multiplier)
	return nil
}

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return ExitUsage
	}
	switch args[0] {
	case "scan":
		return runScan(ctx, args[1:], stderr)
	case "inspect":
		return runInspect(args[1:], stdout, stderr)
	case "extract":
		return runExtract(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		usage(stdout)
		return ExitSuccess
	case "carve", "report", "snapshots":
		fmt.Fprintf(stderr, "chromecarve %s is not implemented in this milestone; run 'chromecarve scan --help' or see docs/SPEC.md\n", args[0])
		return ExitUsage
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		usage(stderr)
		return ExitUsage
	}
}

func runScan(ctx context.Context, args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("scan", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var roots, excludes stringsFlag
	var output, afterText, beforeText string
	var workers int
	var maxScan sizeFlag = 1 << 20
	var progressInterval time.Duration
	var includeHidden, followSymlinks, verbose bool
	flags.Var(&roots, "root", "filesystem root to scan (repeatable)")
	flags.StringVar(&output, "output", "", "JSONL output file")
	flags.StringVar(&afterText, "after", "", "lower RFC3339 modification-time ranking bound")
	flags.StringVar(&beforeText, "before", "", "upper RFC3339 modification-time ranking bound")
	flags.IntVar(&workers, "workers", 0, "number of inspection workers (default 4-8 based on CPUs)")
	flags.Var(&maxScan, "max-content-scan", "maximum bytes inspected per file (supports KiB/MiB/GiB)")
	flags.BoolVar(&includeHidden, "include-hidden", false, "include hidden files and directories")
	flags.BoolVar(&followSymlinks, "follow-symlinks", false, "follow symbolic links with loop-safe root tracking")
	flags.Var(&excludes, "exclude", "exclusion glob (repeatable)")
	flags.DurationVar(&progressInterval, "progress-interval", 5*time.Second, "periodic status interval; 0 disables progress updates")
	flags.BoolVar(&verbose, "verbose", false, "emit debug diagnostics")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: chromecarve scan --root PATH [--root PATH] --output FILE [options]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return ExitSuccess
		}
		return ExitUsage
	}
	if len(roots) == 0 || output == "" {
		fmt.Fprintln(stderr, "scan requires at least one --root and --output")
		flags.Usage()
		return ExitUsage
	}
	if progressInterval < 0 {
		fmt.Fprintln(stderr, "invalid --progress-interval: must be zero or greater")
		return ExitUsage
	}
	after, err := parseTime(afterText)
	if err != nil {
		fmt.Fprintf(stderr, "invalid --after: %v\n", err)
		return ExitUsage
	}
	before, err := parseTime(beforeText)
	if err != nil {
		fmt.Fprintf(stderr, "invalid --before: %v\n", err)
		return ExitUsage
	}
	if chromeRunning() {
		fmt.Fprintln(stderr, "WARN Chrome appears to be running. Results may be inconsistent because cache files can change during scanning. For best results, quit Chrome or scan a copied dataset.")
	}
	file, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		fmt.Fprintf(stderr, "ERROR create inventory %q: %v; choose a writable output path\n", output, err)
		return ExitFatal
	}
	log := func(level, message string) {
		if verbose || level != "debug" {
			fmt.Fprintf(stderr, "%s %s\n", strings.ToUpper(level), message)
		}
	}
	startedAt := time.Now()
	fmt.Fprintf(stderr, "INFO scan started roots=%d output=%q progress_interval=%s\n", len(roots), output, progressInterval)
	var progress func(scanner.Progress)
	if progressInterval > 0 {
		progress = func(update scanner.Progress) {
			fmt.Fprintf(
				stderr,
				"INFO scan progress elapsed=%s scanned=%d candidates=%d permission_errors=%d errors=%d\n",
				update.Elapsed.Round(time.Millisecond),
				update.Scanned,
				update.Candidates,
				update.PermissionErrors,
				update.Errors,
			)
		}
	}
	stats, scanErr := scanner.Scan(ctx, scanner.Config{
		Roots:            roots,
		After:            after,
		Before:           before,
		Workers:          workers,
		MaxContentScan:   int64(maxScan),
		IncludeHidden:    includeHidden,
		FollowSymlinks:   followSymlinks,
		Exclude:          excludes,
		ProgressInterval: progressInterval,
		Progress:         progress,
	}, file, log)
	closeErr := file.Close()
	status := "complete"
	if scanErr != nil || closeErr != nil {
		status = "stopped"
	}
	fmt.Fprintf(stderr, "INFO scan %s elapsed=%s scanned=%d candidates=%d permission_errors=%d errors=%d\n", status, time.Since(startedAt).Round(time.Millisecond), stats.Scanned, stats.Candidates, stats.PermissionErrors, stats.Errors)
	if scanErr != nil {
		fmt.Fprintf(stderr, "ERROR scan failed: %v\n", scanErr)
		return ExitFatal
	}
	if closeErr != nil {
		fmt.Fprintf(stderr, "ERROR close inventory: %v\n", closeErr)
		return ExitFatal
	}
	if stats.PermissionErrors > 0 || stats.Errors > 0 {
		return ExitPartial
	}
	return ExitSuccess
}

func parseTime(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, errors.New("use RFC3339, for example 2026-07-20T00:00:00-07:00")
	}
	return &t, nil
}

func chromeRunning() bool {
	pgrep, err := exec.LookPath("pgrep")
	if err != nil {
		return false
	}
	return exec.Command(pgrep, "-x", "Google Chrome").Run() == nil
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "Usage: chromecarve <command> [options]")
	fmt.Fprintln(w, "Commands: scan, inspect, extract (available); carve, report, snapshots (planned)")
}
