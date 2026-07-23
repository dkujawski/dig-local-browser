# Chromium Simple Cache format notes

Phase 3 follows Chromium's current primary source definitions:

- [`simple_entry_format.h`](https://chromium.googlesource.com/chromium/src/+/refs/heads/main/net/disk_cache/simple/simple_entry_format.h)
- [`simple_backend_version.h`](https://chromium.googlesource.com/chromium/src/+/HEAD/net/disk_cache/simple/simple_backend_version.h)
- [`simple_synchronous_entry.cc`](https://chromium.googlesource.com/chromium/src/+/HEAD/net/disk_cache/simple/simple_synchronous_entry.cc)
- [`http_response_info.cc`](https://chromium.googlesource.com/chromium/src/+/refs/heads/main/net/http/http_response_info.cc)
- [`base/pickle.cc`](https://chromium.googlesource.com/chromium/src/+/main/base/pickle.cc)

The persisted dense entry version is 5. Both `SimpleFileHeader` and
`SimpleFileEOF` occupy 24 bytes. Numeric fields are decoded little-endian for
the initial macOS/Chromium target.

```text
<hash>_0:
  header | key | stream 1 | EOF 1 | stream 0 | [SHA256(key)] | EOF 0

<hash>_1:
  header | key | stream 2 | EOF 2
```

The final stream-0 footer gives its length and indicates whether the key digest
is present. The remaining checked file length determines stream 1. The inner
footer must land exactly between streams 1 and 0 and have the final magic.

Stream 0 commonly stores `HttpResponseInfo` as a Chromium Pickle, not textual
HTTP header lines. The parser reads response-info flags and timestamps until it
reaches the persisted NUL-separated raw header string. It intentionally stops
after the response headers because later fields vary with response flags and are
not needed to locate or classify the body.

Sparse `_s` entries use a different version/layout and remain unsupported.
Entries with unsupported versions return their fixed header as partial evidence
alongside `ErrUnsupportedVersion`.
