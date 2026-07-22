# chromecarve

`chromecarve` is a forensic-style, read-only CLI that searches local files for
recoverable Chromium cache entries associated with images and Reddit resources.
The current milestone implements filesystem discovery and candidate inventory;
structured cache parsing and extraction are the next milestone.

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
file.

## Limitations

Recovery can fail when a response was `no-store`, existed only in memory, was
evicted or overwritten, uses an unsupported cache version, or is truncated.
Images held only in process blobs may not exist on disk. macOS Full Disk Access
restrictions can also hide relevant data. This milestone finds candidates but
does not yet extract their response streams.

## Privacy and safety

Browser storage may contain authentication data, private URLs, conversation
metadata, identifying information, and unrelated cached content. Protect scan
outputs accordingly. The scanner opens sources read-only, makes no network
requests, exports no cookies or credentials, performs no uploads, and emits no
telemetry. Inventory files default to user-only permissions.

See [the specification](docs/SPEC.md), [architecture](docs/architecture.md), and
[recovery workflow](docs/recovery-workflow.md) for design details.
