## Summary

- Establish the Go 1.24 `chromecarve` project and scan command.
- Add bounded, cancellable filesystem discovery, loop-safe symlink following,
  and JSONL candidate inventory.
- Detect and score Chromium layout, Simple Cache magic, Reddit strings, and image signatures.
- Add Make-based build/dependency validation and release artifact automation.
- Document architecture, specification, safety constraints, and recovery workflow.

## Testing

- `make validate`
- `make test-race`
- Cross-platform release builds for the workflow matrix.

## Safety and compatibility

- Source files are opened read-only and inspected with a per-file byte limit.
- Network access is not implemented or used.
- Inventory files are created with user-only permissions.
- The initial target is macOS; release automation also produces Linux and Windows binaries.

## Review notes

- Phase 3 (structured Simple Cache parsing) intentionally follows review of the inventory contract.
- A human maintainer should review this milestone and create the milestone tag after merge.
