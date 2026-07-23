## Summary

- Parse Chromium Simple Cache version-5 headers, keys, footers, and dense streams.
- Validate stream bounds, PersistentHash, CRC-32, optional key SHA-256, and body SHA-256.
- Decode Chromium Pickle response metadata into HTTP status, headers, MIME type,
  and content encoding.
- Add `chromecarve inspect` and activate valid-cache-key scanner scoring.
- Make command-specific `--help` return a successful exit status.
- Document the binary layout, parser limits, and Phase 3 architecture.

## Testing

- `make validate`
- `make test-race`
- `go test -fuzz=FuzzSimpleCacheParser -fuzztime=2s ./internal/simplecache`
- Cross-platform release builds

## Safety and compatibility

- Source cache files are opened read-only.
- All offsets use checked arithmetic and body data is exposed without allocation.
- Keys, HTTP metadata, and integrity scans have explicit limits.
- Dense entry version 5 is supported; sparse and unsupported versions return
  actionable typed errors.

## Review notes

- Phase 4 can consume the stream-1 `io.SectionReader` for raw and decoded extraction.
- A human maintainer should review this milestone and create its tag after merge.
