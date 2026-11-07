# Version-source filter ports

C# baseline: `14d222235b7dabb7033473528a76fd18478a9217`.

All 12 methods in MinDateVersionFilterTests, ShaVersionFilterTests and PathFilterTests
retain their original exclusion result, null/nonblank reason assertion, null base
version guard and null commit behavior. Fixed UTC commit instants replace the
original relative clock/mock values. Original path strings and matching/mismatching
SHA relationships are preserved. Go errors replace null argument exceptions.

| Requirement | Evidence |
| --- | --- |
| All original filter assertions and guard cases | `TestCSharpVersionFilters` |
| Before/at/after minimum date | `TestVersionFilterBoundaries/date--1ns`, `TestVersionFilterBoundaries/date-0s`, `TestVersionFilterBoundaries/date-1ns` |
| Case-insensitive SHA prefixes, empty list/prefix and mismatch | `TestVersionFilterBoundaries` |
| Every diff path must match, empty diffs and regex options/errors | `TestVersionFilterBoundaries` |

The reusable production filter APIs cover both base-version and direct-commit
inputs. The calculator's CLI/configuration ignore wiring is still pending; this
batch does not claim ignored-source behavior is enabled by a YAML configuration.
The source-port harness does not execute NUnit. Windows stays pending.

Implementation checkpoints: `0060f88d2`, `5c7e85eec`. Both macOS arm64 and
Ubuntu 26.04.1 arm64 passed **37,731 regular tests/subtests in 11 packages**, with
zero failures/skips. The complete original cases and supplementary boundaries
ran on both OS. Fresh regression probes also passed 25 repository/merge JSON,
88 global-mode JSON, 87 calculation JSON and six configuration cases per OS.
Those process probes verify existing calculation behavior; ignore wiring remains
pending. The real checkout still matches all 33 fields/types: 7.0.0-alpha.1+739.

`CSHARP-VERSION-FILTER-EVIDENCE.json` binds production source to 5c7e85eec,
binaries, corpus files, logs, reports and archived Linux inputs. Source extraction
is deterministic. The Ubuntu container had no network access or host mounts.

Verified unit/component methods: **576 / 1,516 (37.99%)**, with **789 more**
required to reach 1,365 (90%). Integration remains 38 / 662; overall mapping
is 614 / 2,178 (28.19%). These are source-method ports, not statement coverage or
full replacement compatibility. CLI ignore wiring and Windows remain pending.
