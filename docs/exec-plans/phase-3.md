# Phase 3: Chromium Simple Cache parser

This milestone adds a bounded parser for Chromium Simple Cache entry files. It
uses Chromium's persisted version-5 header/footer layout, locates streams from
validated file boundaries, decodes HTTP response metadata stored as a Chromium
`base::Pickle`, and exposes the result through `chromecarve inspect`.

## Primary format references

- Chromium `simple_entry_format.h` for file order and persisted structures.
- Chromium `simple_backend_version.h` for entry version 5.
- Chromium `simple_synchronous_entry.cc` for stream offset calculations,
  footer validation, CRC behavior, and optional key SHA-256 placement.
- Chromium `http_response_info.cc`, `http_response_headers.cc`, and
  `base/pickle.cc` for response metadata serialization.

## Plan

- Build synthetic combined-stream and stream-2 fixtures.
- Add red tests for valid parsing, typed failure modes, hostile lengths,
  truncated entries, CRC/key validation, response headers, and fuzz safety.
- Implement fixed-size header/footer decoding with checked offset arithmetic.
- Return partial header/key evidence with typed errors where safe.
- Expose body data through `io.SectionReader` without loading it into memory.
- Parse bounded HTTP metadata and preserve warnings when metadata is recoverable
  but imperfect.
- Implement human-readable `inspect` output.
- Update specification, format notes, README, changelog, and PR record.
- Run unit, race, vet, fuzz smoke, and cross-platform build validation.

Phase 4 will consume the parsed stream boundaries for raw/decoded extraction.
