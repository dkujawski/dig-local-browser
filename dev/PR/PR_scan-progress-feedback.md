## Summary

- Report when a scan starts and completes.
- Emit throttled scan progress with elapsed time, scanned files, candidates,
  permission errors, and other errors.
- Add `--progress-interval` to configure or disable periodic status updates.
- Document the scan progress contract.

## Testing

- `go test ./internal/scanner ./internal/cli`
- `make validate`
- `make test-race`

## Safety and compatibility

- Status remains on stderr, so JSONL inventory output is unchanged.
- Periodic updates default to five seconds and can be disabled with
  `--progress-interval 0`.
- Scanning remains read-only and makes no network requests.

## Review notes

- A human maintainer should review the change and create the milestone tag after
  merge.
