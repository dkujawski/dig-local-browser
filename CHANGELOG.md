# Changelog

## Unreleased

- Allow `chromecarve inspect` to inspect candidate paths directly from scan
  JSONL output files.
- Add configurable status and progress feedback for long-running scans.
- Add safe Chromium Simple Cache version-5 parsing, HTTP response metadata
  decoding, integrity checks, body stream boundaries, and `chromecarve inspect`.
- Score scanner candidates when a bounded cache key contains a valid URL.
- Return a successful exit status for command-specific `--help` output.
- Fix release artifact attachment when the upload job runs without a repository checkout.
- Add the initial `chromecarve scan` command with bounded concurrent discovery,
  explainable candidate scoring, and privacy-conscious JSONL output.
- Add Make-based dependency, test, vet, and build orchestration.
- Add release automation for macOS, Linux, and Windows archives.
