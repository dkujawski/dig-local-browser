## Summary

- Add `chromecarve inspect --input FILE` batch inspection for scan JSONL output.
- Continue past malformed JSONL records and candidate-specific inspection errors.
- Document the scan-to-inspect workflow and partial-success behavior.

## Testing

- `make validate`

## Safety and compatibility

- Existing single-path inspection remains unchanged.
- JSONL files and referenced cache entries are opened read-only.
- Batch inspection returns exit code 1 when one or more records cannot be inspected.

## Review notes

- Please tag the reviewed milestone after merge.
