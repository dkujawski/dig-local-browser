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

The future parser belongs in `internal/simplecache` and will consume source files
from inventory records. Extraction will consume parser streams, never scanner
buffers, so the discovery byte limit cannot truncate recovered payloads.
