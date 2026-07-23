# Recovery workflow

1. Quit Chrome or copy the cache data before scanning.
2. Protect the working directory; browser data can reveal private URLs and identity data.
3. Scan likely roots with a narrow time window as a ranking hint.
4. Review the JSONL candidate paths and signals.
5. Inspect high-confidence entries before extraction.
6. Extract validated images into a private output directory.

```bash
chromecarve scan \
  --root "$HOME" \
  --root /private/var/folders \
  --after 2026-07-20T00:00:00-07:00 \
  --before 2026-07-23T00:00:00-07:00 \
  --output findings.jsonl

chromecarve inspect --input findings.jsonl

mkdir -m 700 recovered
chromecarve extract \
  --input findings.jsonl \
  --output recovered \
  > extraction-results.jsonl
```

Extraction results contain source paths, URLs when available, raw and decoded
SHA-256 values, image types, artifact paths, and deduplication status. Encoded
responses preserve both the original `.raw` body and the decoded image. Protect
the result JSONL and recovered files as sensitive browser data.

Full Disk Access restrictions may prevent inspection of relevant directories.
The scanner reports and continues past those paths. For APFS/Time Machine data,
list local snapshots with `tmutil listlocalsnapshots /`, mount or expose the
desired snapshot through supported macOS tooling, then scan that read-only root.
