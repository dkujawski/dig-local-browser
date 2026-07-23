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

## Deferred milestones

- Phase 3: safe Chromium Simple Cache parsing and stream bounds validation.
- Phase 4: raw/decoded image extraction, encoding support, hashing, and deduplication.
- Phase 5: Markdown reports.
- Phase 6: container-aware fallback carving.
- Phase 7: fuzzing, real copied-cache validation, and format hardening.

All milestones remain read-only with respect to source browser data. Network
requests, cookie extraction, credentials, telemetry, and automatic uploads are
out of scope.
