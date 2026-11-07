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

Platform gates are running; all 12 mappings remain pending verification.
