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
records. Extraction will consume parser streams, never scanner buffers, so the
discovery byte limit cannot truncate recovered payloads.

## Simple Cache parser

The parser separates fixed-record decoding, Chromium PersistentHash, Chromium
Pickle/HTTP metadata parsing, and public entry models. Checked arithmetic derives
every stream boundary from file size, header/key size, footer records, and the
stream-0 length. Bodies are exposed with `io.SectionReader`; only bounded keys
and HTTP metadata are allocated. `ParseFile` uses `_0`/`_1` suffixes to enforce
layout, while `Parse` supports unnamed forensic inputs through structural
auto-detection.
