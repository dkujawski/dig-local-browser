// Package scanner implements the bounded, read-only discovery pipeline.
package scanner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dkujawski/dig-local-browser/internal/discovery"
	"github.com/dkujawski/dig-local-browser/internal/fsutil"
	"github.com/dkujawski/dig-local-browser/internal/manifest"
	"github.com/dkujawski/dig-local-browser/internal/reddit"
	"github.com/dkujawski/dig-local-browser/internal/scoring"
	"github.com/dkujawski/dig-local-browser/internal/signatures"
	"github.com/dkujawski/dig-local-browser/internal/simplecache"
)

const defaultMaxContentScan int64 = 1024 * 1024

type Config struct {
	Roots            []string
	After            *time.Time
	Before           *time.Time
	Workers          int
	MaxContentScan   int64
	IncludeHidden    bool
	FollowSymlinks   bool
	Exclude          []string
	ProgressInterval time.Duration
	Progress         func(Progress)
}

type Stats struct {
	Scanned          int64
	Candidates       int64
	PermissionErrors int64
	Errors           int64
}

type Progress struct {
	Stats
	Elapsed time.Duration
}

type Logger func(level, message string)

type inspectResult struct {
	candidate *manifest.Candidate
	err       error
}

func (c *Config) defaults() error {
	if len(c.Roots) == 0 {
		return errors.New("at least one --root is required")
	}
	if c.Workers == 0 {
		c.Workers = min(max(runtime.NumCPU(), 4), 8)
	}
	if c.Workers < 1 {
		return errors.New("--workers must be at least 1")
	}
	if c.MaxContentScan == 0 {
		c.MaxContentScan = defaultMaxContentScan
	}
	if c.MaxContentScan < 8 {
		return errors.New("--max-content-scan must be at least 8 bytes")
	}
	if c.After != nil && c.Before != nil && c.After.After(*c.Before) {
		return errors.New("--after must not be later than --before")
	}
	if c.Progress != nil && c.ProgressInterval <= 0 {
		return errors.New("progress interval must be greater than zero")
	}
	return nil
}

// Scan walks roots, inspects files with a bounded worker pool, and serializes
// candidates through a single JSONL writer. Source files are only opened read-only.
func Scan(parent context.Context, cfg Config, output io.Writer, log Logger) (Stats, error) {
	if err := cfg.defaults(); err != nil {
		return Stats{}, err
	}
	startedAt := time.Now()
	lastProgress := startedAt
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	paths := make(chan string, cfg.Workers*2)
	results := make(chan inspectResult, cfg.Workers*2)
	var scanned, permissionErrors, scanErrors atomic.Int64

	var workers sync.WaitGroup
	for range cfg.Workers {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for path := range paths {
				if ctx.Err() != nil {
					return
				}
				scanned.Add(1)
				candidate, err := inspect(path, cfg)
				select {
				case results <- inspectResult{candidate: candidate, err: err}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	walkDone := make(chan error, 1)
	go func() {
		defer close(paths)
		walkDone <- walk(ctx, cfg, paths, &permissionErrors, &scanErrors, log)
	}()
	go func() { workers.Wait(); close(results) }()

	writer := manifest.NewWriter(output)
	var candidates int64
	var fatal error
	for result := range results {
		if result.err != nil {
			if errors.Is(result.err, fs.ErrPermission) {
				permissionErrors.Add(1)
			} else {
				scanErrors.Add(1)
			}
			if log != nil {
				log("warn", result.err.Error())
			}
		}
		if result.candidate == nil {
			reportProgress(cfg, startedAt, &lastProgress, scanned.Load(), candidates, permissionErrors.Load(), scanErrors.Load())
			continue
		}
		if err := writer.Write(*result.candidate); err != nil {
			fatal = fmt.Errorf("write inventory: %w", err)
			cancel()
			continue
		}
		candidates++
		reportProgress(cfg, startedAt, &lastProgress, scanned.Load(), candidates, permissionErrors.Load(), scanErrors.Load())
	}
	walkErr := <-walkDone
	stats := Stats{Scanned: scanned.Load(), Candidates: candidates, PermissionErrors: permissionErrors.Load(), Errors: scanErrors.Load()}
	if fatal != nil {
		return stats, fatal
	}
	if walkErr != nil {
		return stats, walkErr
	}
	if err := parent.Err(); err != nil {
		return stats, err
	}
	return stats, nil
}

func reportProgress(cfg Config, startedAt time.Time, lastProgress *time.Time, scanned, candidates, permissionErrors, scanErrors int64) {
	if cfg.Progress == nil {
		return
	}
	now := time.Now()
	if now.Sub(*lastProgress) < cfg.ProgressInterval {
		return
	}
	*lastProgress = now
	cfg.Progress(Progress{
		Stats: Stats{
			Scanned:          scanned,
			Candidates:       candidates,
			PermissionErrors: permissionErrors,
			Errors:           scanErrors,
		},
		Elapsed: now.Sub(startedAt),
	})
}

func walk(ctx context.Context, cfg Config, paths chan<- string, permissionErrors, scanErrors *atomic.Int64, log Logger) error {
	pending := append([]string(nil), cfg.Roots...)
	seenRoots := make(map[string]bool)
	for len(pending) > 0 {
		root := pending[0]
		pending = pending[1:]
		if err := ctx.Err(); err != nil {
			return err
		}
		root = filepath.Clean(root)
		if cfg.FollowSymlinks {
			resolved, err := filepath.EvalSymlinks(root)
			if err == nil {
				root = resolved
			}
		}
		canonical, err := filepath.Abs(root)
		if err != nil {
			return fmt.Errorf("resolve root %q: %w", root, err)
		}
		if seenRoots[canonical] {
			continue
		}
		seenRoots[canonical] = true
		err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if walkErr != nil {
				if errors.Is(walkErr, fs.ErrPermission) {
					permissionErrors.Add(1)
					if log != nil {
						log("warn", fmt.Sprintf("path=%q error=%q", path, walkErr))
					}
					if entry != nil && entry.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				scanErrors.Add(1)
				if log != nil {
					log("warn", fmt.Sprintf("path=%q error=%q", path, walkErr))
				}
				return nil
			}
			if path != root && !cfg.IncludeHidden && strings.HasPrefix(entry.Name(), ".") {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if excluded(path, cfg.Exclude) {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 {
				if !cfg.FollowSymlinks {
					return nil
				}
				info, err := os.Stat(path)
				if err != nil {
					return nil
				}
				if info.IsDir() {
					resolved, err := filepath.EvalSymlinks(path)
					if err == nil && !coveredByRoot(resolved, canonical) && !seenRoots[resolved] {
						pending = append(pending, resolved)
					}
					return nil
				}
				if !info.Mode().IsRegular() {
					return nil
				}
			} else if !entry.Type().IsRegular() {
				return nil
			}
			select {
			case paths <- path:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		if err != nil {
			return fmt.Errorf("walk root %q: %w", root, err)
		}
	}
	return nil
}

func coveredByRoot(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func excluded(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if ok, _ := filepath.Match(pattern, path); ok {
			return true
		}
		if ok, _ := filepath.Match(pattern, filepath.Base(path)); ok {
			return true
		}
	}
	return false
}

func inspect(path string, cfg Config) (*manifest.Candidate, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %q: %w", path, err)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", path, err)
	}
	defer file.Close()
	limit := min(info.Size(), cfg.MaxContentScan)
	data := make([]byte, limit)
	n, readErr := io.ReadFull(file, data)
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return nil, fmt.Errorf("read %q: %w", path, readErr)
	}
	data = data[:n]
	simple, detectErr := simplecache.Detect(file, info.Size())
	if errors.Is(detectErr, simplecache.ErrTruncatedEntry) {
		detectErr = nil
	}
	pathSignals := discovery.ClassifyPath(path)
	contentSignals := reddit.FindSignals(data)
	imageSignatures := signatures.Find(data)
	inWindow := (cfg.After != nil || cfg.Before != nil) && (cfg.After == nil || !info.ModTime().Before(*cfg.After)) && (cfg.Before == nil || !info.ModTime().After(*cfg.Before))
	score := scoring.Evaluate(scoring.Features{SimpleCache: simple, CacheStructure: pathSignals.CacheStructure, RedditURL: contentSignals.Reddit, ImageContentType: contentSignals.ImageContentType, ImageSignature: len(imageSignatures) > 0, ChromePath: pathSignals.ChromePath, ServiceWorker: pathSignals.ServiceWorker, InTimeWindow: inWindow, EntryFilename: pathSignals.EntryFilename, Unrelated: pathSignals.Unrelated})
	if score.Confidence <= 0 {
		return nil, detectErr
	}
	device, inode, created := fsutil.Identity(info)
	candidate := &manifest.Candidate{Path: path, Size: info.Size(), ModifiedAt: info.ModTime(), CreatedAt: created, Device: device, Inode: inode, SimpleCache: simple, CacheStructure: pathSignals.CacheStructure, ImageSignatures: imageSignatures, MatchedStrings: contentSignals.Matched, Confidence: score.Confidence, Signals: score.Signals, Errors: []string{}}
	if detectErr != nil {
		candidate.Errors = append(candidate.Errors, detectErr.Error())
	}
	return candidate, detectErr
}
