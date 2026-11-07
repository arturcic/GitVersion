# Dual-backend repository parity

Source checkpoint `33f4d8525` ports all 19 original methods. The test-only
oracle runs both pinned C# adapters against each deterministic go-git
fixture and records a result only when the adapters agree. Go then compares
that result with its repository reads, preserving the sequence order.
Temporary paths are expressed relative to the fixture root.

The comparisons include reference and branch enumeration, tracking and
remotes, commit metadata, tags, status, and repository layout. The original
matrices contain 48 ordered walk queries, 49 criss-cross merge bases, and
25 octopus merge bases. The fixtures also cover an empty repository,
detached HEAD, a bare repository, mixed loose and packed refs, a depth-one
clone, a linked worktree, and an index in version 4 format.

macOS arm64 and offline Ubuntu 26.04 arm64 each passed 39,935 tests and
subtests across 13 packages. All 19 live dual-backend comparisons passed
on each OS. The actual checkout matches all 33 C# JSON fields at
`7.0.0-alpha.1+739`, using a snapshot on Ubuntu. Production source hashes
match the previous checkpoint, which retains the earlier process evidence.

The verified inventory reaches 1,187 of 1,516 unit and component methods
(78.30%). Another 178 methods are needed for 90%. Windows validation is pending.

`CSHARP-DUAL-BACKEND-EVIDENCE.json` records the source and executable hashes,
comparison matrices, process reports, and checkout snapshot.
