# Architecture

The initial pipeline is:

```text
root walker -> bounded path queue -> inspection workers -> result queue -> JSONL writer
```

`internal/discovery`, `reddit`, `signatures`, and `simplecache` own independent
detection rules. `internal/scoring` maps their boolean features to explainable
weights. `internal/scanner` handles filesystem and concurrency concerns, while
`internal/manifest` owns the stable record format. `internal/cli` adapts command
flags, signals, logging, and exit codes to the scanner.

The parser in `internal/simplecache` consumes source files from inventory
records. Extraction consumes parser streams, never scanner buffers, so the
discovery byte limit cannot truncate recovered payloads.

## Simple Cache parser

The parser separates fixed-record decoding, Chromium PersistentHash, Chromium
Pickle/HTTP metadata parsing, and public entry models. Checked arithmetic derives
every stream boundary from file size, header/key size, footer records, and the
stream-0 length. Bodies are exposed with `io.SectionReader`; only bounded keys
and HTTP metadata are allocated. `ParseFile` uses `_0`/`_1` suffixes to enforce
layout, while `Parse` supports unnamed forensic inputs through structural
auto-detection.

## Image extractor

`internal/extractor` keeps each source cache file open while parsing and copying
its validated stream-1 range. Raw and decoded data flow through bounded readers
and SHA-256 hashers into private staging files. HTTP content decoders are
composed in reverse header order. Signature validation occurs before artifacts
are installed.

Artifact names are content digests. A hard-link installation step provides
atomic no-replace behavior within the output directory; a matching existing
artifact is reused, while mismatched content is reported instead of
overwritten. `internal/cli` owns single-path and scan-JSONL orchestration and
keeps JSONL results separate from diagnostics.
