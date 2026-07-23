# chromecarve

`chromecarve` is a forensic-style, read-only CLI that searches local files for
recoverable Chromium cache entries associated with images and Reddit resources.
Filesystem discovery, candidate inventory, structured cache parsing, and
validated image extraction are available.

## Build and validate

Go 1.24 or newer is required.

```bash
make deps
make validate
make build
```

The binary is written to `build/chromecarve`.

## Scan

Quit Chrome first, or scan a copied dataset for consistent results.

```bash
./build/chromecarve scan \
  --root "$HOME" \
  --root /private/var/folders \
  --after 2026-07-20T00:00:00-07:00 \
  --before 2026-07-23T00:00:00-07:00 \
  --output findings.jsonl
```

Run `chromecarve scan --help` for worker, byte-limit, hidden-file, symlink,
exclusion, and diagnostic options. Time bounds are confidence signals, not hard
filters. Logs go to stderr and machine-readable records go only to the output
file. Scans report when they start, emit progress every five seconds, and print
a completion summary. Use `--progress-interval DURATION` to change the update
frequency or `--progress-interval 0` to disable periodic updates.

## Inspect a cache entry

```bash
./build/chromecarve inspect \
  "/path/to/Cache/Cache_Data/0123456789abcdef_0"
```

Inspect every candidate path from a scan output file:

```bash
./build/chromecarve inspect --input findings.jsonl
```

Inspection reports the cache version and key, URL, checked stream offsets and
lengths, HTTP response headers, MIME type, content encoding, body SHA-256, CRC
status, and non-fatal parsing warnings. JSONL batch inspection continues after
malformed records or candidate-specific errors and returns a partial-success
status when any record could not be inspected. Source files remain read-only.

## Extract images

Extract one inspected cache entry:

```bash
./build/chromecarve extract \
  --output recovered \
  "/path/to/Cache/Cache_Data/0123456789abcdef_0"
```

Or process every candidate path from scan JSONL:

```bash
./build/chromecarve extract \
  --input findings.jsonl \
  --output recovered \
  > extraction-results.jsonl
```

The command supports identity, gzip, deflate, Brotli, and stacked content
encodings. It retains encoded raw bodies, validates decoded image signatures,
names artifacts by SHA-256, and deduplicates identical images. Decoded images
are limited to 256 MiB by default; use `--max-decoded-size SIZE` to adjust the
limit. Artifacts are private mode `0600`, source cache files remain read-only,
JSONL results go to stdout, and diagnostics go to stderr.

## Limitations

Recovery can fail when a response was `no-store`, existed only in memory, was
evicted or overwritten, uses an unsupported cache version, or is truncated.
Images held only in process blobs may not exist on disk. macOS Full Disk Access
restrictions can also hide relevant data. Entry version 5 dense files are
supported; sparse entries and other versions are reported but not parsed.
Zstandard and other unlisted content encodings are reported but not decoded.

## Privacy and safety

Browser storage may contain authentication data, private URLs, conversation
metadata, identifying information, and unrelated cached content. Protect scan
outputs accordingly. The scanner opens sources read-only, makes no network
requests, exports no cookies or credentials, performs no uploads, and emits no
telemetry. Inventory files default to user-only permissions.

See [the specification](docs/SPEC.md), [architecture](docs/architecture.md),
[cache format notes](docs/cache-format-notes.md), and
[recovery workflow](docs/recovery-workflow.md) for design details.
