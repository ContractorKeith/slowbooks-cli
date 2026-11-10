# Changelog

## 0.1.1

- Show balance due alongside totals in invoice and bill tables.
- Store payment keys as customer payment references or bill payment check numbers,
  preserving explicit data fields and showing defaults in dry-run.
- Name voided records and their numbers or IDs, with a matching show command.
- Add `ops find --all`, truncation summaries and concise matches; keep full
  schemas in `ops describe`.
- Document public release downloads, checksum verification and Go installation.

## 0.1.0

- Initial CLI: connection profiles, named bookkeeping reads and writes, reports,
  downloads, server operation discovery, JSON envelopes and agent reference.
- Verified against SlowBooks Pro 2.31.10 across Linux, macOS and Windows.
