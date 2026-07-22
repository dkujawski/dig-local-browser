# Phase 1-2: Scanner and Candidate Inventory

This milestone establishes the `chromecarve` command and a safe, bounded,
read-only scanner. The scanner walks arbitrary roots, inspects a bounded prefix
of each regular file, scores independent Chromium, Reddit, and image signals,
and writes explainable JSONL candidate records through one writer.

## Plan

- Define stable inventory models and JSONL encoding.
- Add detectors for Simple Cache magic, cache paths, Reddit strings, escaped
  URLs, and image containers.
- Centralize candidate weights in `internal/scoring`.
- Build a context-aware producer/worker/collector pipeline with bounded queues.
- Expose the pipeline through `chromecarve scan` with repeatable roots and
  exclusions, time-ranking options, logging, and signal handling.
- Verify unit, integration, cancellation, race, and vet behavior.

Phase 3 will consume this inventory and add the full Simple Cache parser only
after this JSONL contract is reviewed and stable.
