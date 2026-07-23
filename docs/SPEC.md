# chromecarve specification

## Milestone 1: discovery and candidate inventory

`chromecarve scan` accepts repeatable arbitrary roots and writes one JSON object
per candidate. Discovery must not assume a Chrome profile location. It combines
independent filesystem-layout, Simple Cache magic, Reddit string, HTTP image
content-type, filename, time, and image-container signals. Each record preserves
the individual signals and score.

The scanner uses one filesystem producer, a bounded worker pool, and one result
consumer. It reads at most `--max-content-scan` bytes from any file, does not
follow symlinks unless requested, continues after inaccessible paths, and
propagates cancellation. Modification bounds affect ranking rather than exclude
files. JSONL output is separate from stderr diagnostics and is created mode
`0600`.

The command writes scan lifecycle feedback to stderr: an immediate start
message, progress snapshots at a configurable interval, and a completion
summary. Progress snapshots report elapsed time, files scanned, candidates,
permission errors, and other errors. Periodic feedback defaults to every five
seconds and can be disabled without affecting the final summary.

The scoring policy is centralized in `internal/scoring` and follows the weights
in the project brief. A record is a candidate when its final score is positive.

## Milestone 2: Simple Cache parsing

`internal/simplecache` parses persisted entry version 5 using 24-byte
little-endian header and footer records. It validates bounded key lengths,
PersistentHash values, final magic values, stream offsets, optional CRC-32
values, and optional key SHA-256 values. Combined `_0` files expose stream 0
(HTTP metadata) and stream 1 (body); `_1` files expose stream 2.

HTTP response metadata is decoded from Chromium's `base::Pickle` representation.
The parser returns the status line, status code, repeated headers, MIME type,
and content encoding. Body data remains an `io.SectionReader`; the parser does
not allocate based on body length. CRC and SHA-256 validation is capped at 64
MiB per stream and records a warning when skipped. HTTP metadata allocation is
capped at 16 MiB and cache keys at 1 MiB.

`chromecarve inspect PATH` presents parsed fields, stream boundaries, hashes,
headers, and warnings. `chromecarve inspect --input FILE` reads candidate paths
from scan JSONL in file order. Batch inspection skips malformed records, keeps
processing later records, and reports partial success if any record cannot be
decoded or inspected. Filename-aware parsing enforces `_0` and `_1` layouts;
the reader-only API uses conservative layout auto-detection.

## Phase 4: image extraction

`chromecarve extract --output DIR PATH` extracts the response body from one
parsed combined entry. `chromecarve extract --input FILE --output DIR` processes
candidate paths from scan JSONL in file order, skips malformed or failed
records, and reports partial success when needed.

The extractor hashes the raw body with SHA-256, reverses identity, gzip,
deflate, Brotli, and stacked HTTP content encodings, and hashes the decoded
bytes independently. Decoded output defaults to a 256 MiB limit that callers
can change with `--max-decoded-size`. Unsupported encodings, decode failures,
and payloads beyond the limit produce actionable errors without leaving staging
files behind.

Decoded bytes must begin with a recognized JPEG, PNG, GIF, WebP, AVIF, HEIC, or
HEIF signature. MIME metadata remains advisory. Digest-derived artifact names
avoid unsafe cache-key or URL filenames and deduplicate identical decoded
images. Encoded bodies also retain a `.raw` artifact; identity bodies use the
image artifact as both raw and decoded output. Files are mode `0600`, synced,
and installed without replacing existing content. Extraction results are
written as JSONL to stdout, separately from stderr diagnostics.

## Deferred milestones

- Phase 5: Markdown reports.
- Phase 6: container-aware fallback carving.
- Phase 7: fuzzing, real copied-cache validation, and format hardening.

All milestones remain read-only with respect to source browser data. Network
requests, cookie extraction, credentials, telemetry, and automatic uploads are
out of scope.
