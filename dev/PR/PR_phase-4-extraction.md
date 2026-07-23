## Summary

- Add `chromecarve extract` for single cache entries and scan JSONL inputs.
- Stream raw and decoded image artifacts with bounded gzip, deflate, Brotli,
  and stacked content-encoding support.
- Validate image signatures, name artifacts by SHA-256, retain encoded raw
  bodies, and deduplicate decoded images without replacing existing content.
- Document the Phase 4 design, workflow, limitations, and user-visible change.

## Testing

- `go test ./internal/extractor ./internal/simplecache`
- `go test ./internal/cli`
- `go test ./internal/signatures ./internal/extractor ./internal/cli`
- `make validate`
- `make test-race`
- `go test ./internal/simplecache -run='^$' -fuzz=FuzzSimpleCacheParser -fuzztime=2s`
- `make build`
- Cross-compile `./cmd/chromecarve` for Darwin arm64, Linux amd64, and Windows
  amd64.

## Safety and compatibility

Cache sources are opened read-only. Decoded output is bounded to 256 MiB by
default. Artifacts use mode `0600`, are synced before installation, and are
installed with atomic no-replace behavior. Unsupported encodings and invalid
image payloads produce actionable errors without leaving staging files.

The new Brotli decoder is a pure-Go dependency. Existing scan and inspect
interfaces are unchanged.

## Review notes

Please review the artifact naming and raw-retention policy: identity bodies use
one image artifact for both raw and decoded paths, while encoded bodies retain a
separate digest-named `.raw` artifact.
