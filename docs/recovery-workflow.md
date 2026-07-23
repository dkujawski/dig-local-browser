# Recovery workflow

1. Quit Chrome or copy the cache data before scanning.
2. Protect the working directory; browser data can reveal private URLs and identity data.
3. Scan likely roots with a narrow time window as a ranking hint.
4. Review the JSONL candidate paths and signals.
5. Inspect high-confidence entries before extraction.
6. Use the structured extraction command after Phase 4 is available.

```bash
chromecarve scan \
  --root "$HOME" \
  --root /private/var/folders \
  --after 2026-07-20T00:00:00-07:00 \
  --before 2026-07-23T00:00:00-07:00 \
  --output findings.jsonl

chromecarve inspect \
  "/path/from/findings/0123456789abcdef_0"
```

Full Disk Access restrictions may prevent inspection of relevant directories.
The scanner reports and continues past those paths. For APFS/Time Machine data,
list local snapshots with `tmutil listlocalsnapshots /`, mount or expose the
desired snapshot through supported macOS tooling, then scan that read-only root.
