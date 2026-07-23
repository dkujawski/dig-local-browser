## Summary

Restore the missing closing braces in the scanner progress test so the scanner test package compiles and the full validation suite can run.

## Testing

- `go test ./internal/scanner`
- `make`

## Safety and compatibility

This test-only syntax correction does not change runtime behavior, compatibility, or migration requirements.

## Review notes

No follow-up work or maintainer decisions are required.
