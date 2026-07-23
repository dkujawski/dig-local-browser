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
headers, and warnings. Filename-aware parsing enforces `_0` and `_1` layouts;
the reader-only API uses conservative layout auto-detection.

## Deferred milestones

- Phase 4: raw/decoded image extraction, encoding support, hashing, and deduplication.
- Phase 5: Markdown reports.
- Phase 6: container-aware fallback carving.
- Phase 7: fuzzing, real copied-cache validation, and format hardening.

All milestones remain read-only with respect to source browser data. Network
requests, cookie extraction, credentials, telemetry, and automatic uploads are
out of scope.
