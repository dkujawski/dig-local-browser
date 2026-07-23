# Phase 4: image extraction, decoding, hashing, and deduplication

This milestone adds a read-only extraction workflow for image response bodies
already located by the Chromium Simple Cache parser. The command writes
forensic raw payloads and decoded image files to a caller-selected directory,
without modifying or renaming source cache entries.

## Design decisions

- `chromecarve extract PATH --output DIR` extracts one combined `_0` entry.
- `chromecarve extract --input FILE --output DIR` processes candidate paths
  from scan JSONL in file order and continues after record-specific failures.
- The raw HTTP response body is hashed while it is copied. Supported
  `Content-Encoding` values are identity, gzip, deflate, and Brotli; stacked
  encodings are decoded in reverse order.
- Decoded output is bounded by `--max-decoded-size` (256 MiB by default) to
  prevent compressed payloads from consuming unbounded disk space.
- A decoded payload must begin with a recognized JPEG, PNG, GIF, WebP, AVIF,
  HEIC, or HEIF signature. MIME metadata is advisory and does not override the
  bytes.
- Artifact names use SHA-256 digests instead of cache keys or URLs. This avoids
  unsafe filenames and deduplicates decoded images across cache entries.
- Raw bodies are retained separately when an HTTP content encoding is present.
  Identity bodies use the decoded image artifact as their raw representation,
  avoiding a duplicate file with identical bytes.
- Files are staged in the destination directory with mode `0600`, synced, and
  atomically renamed. Existing digest-named artifacts are reused only when
  their content hash matches.
- JSONL results are written to stdout. Diagnostics go to stderr. Batch runs
  return partial success when some records fail, matching `inspect` behavior.

## Plan

- Add red unit tests for identity, gzip, deflate, Brotli, stacked encodings,
  unsupported encodings, decode-size limits, invalid image data, deduplication,
  and source preservation.
- Implement a streaming extractor around an open cache file and the parser's
  validated stream-1 offsets.
- Add red CLI tests for single-path and scan-JSONL workflows, usage errors,
  continuation after failures, and actionable diagnostics.
- Implement `chromecarve extract` flags, JSONL output records, and exit-code
  behavior.
- Update the specification, architecture, recovery workflow, README, changelog,
  and PR record.
- Run formatting, unit, race, vet, build, and cross-platform compilation
  validation.

Phase 5 can consume extraction JSONL to build Markdown reports without
re-reading or trusting source cache filenames.
