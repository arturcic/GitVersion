# Merge-message source ports

Pinned C# baseline: `14d222235b7dabb7033473528a76fd18478a9217`.
All 15 MergeMessageTests methods and 49 cases retain original assertions for
format name, target/source branch, pull-request number/flag, semantic version,
null/empty values and null-argument errors. Builder dictionaries become ordered
Go format lists. EmptyTagPrefix retains all three original arguments: its message
matches no pattern, so the prefix does not participate in a parse.

The production calculator now uses the shared parser for Default, SmartGit,
Bitbucket Server/Cloud/v7, GitHub, remote-tracking and Azure DevOps messages.
Configured patterns run first in order. Named captures distinguish absence from
an empty match. Integer parsing uses the C# Int32 range. Regexes retain original
case options, and version extraction uses the existing strict branch parser.

| Requirement | Evidence |
| --- | --- |
| All original format, capture, null and precedence assertions | `TestCSharpMergeMessages` |
| Custom casing, malformed regex, empty captures and Int32 overflow | `TestMergeMessagePatternBoundaries` |
| Version output through eight built-in merge formats | `TestPlatformRepositoryCompatibilityParity` |

Implementation checkpoints: `b3d5fe6c8` and `9956af7f7`. Both macOS arm64 and
Ubuntu 26.04.1 arm64 passed 37,688 regular tests/subtests in 11 packages, with zero
failures/skips. Fresh process gates on each OS passed 25 repository/merge-format
JSON comparisons (eight new formats), 88 global-mode JSON comparisons, 87 prior
calculation JSON comparisons and six configuration checks. The real repository
still matches all 33 values/types in C# and Go: 7.0.0-alpha.1+739.

`CSHARP-MERGE-MESSAGE-EVIDENCE.json` records source, binary, report and archive
hashes, tied to 9956af7f7 and the actual Linux input bundle. Extraction is
deterministic. Linux ran without network access or host mounts. Logs and process
outputs are retained locally. Original NUnit is not run by this source-port harness.

Verified unit/component total: **564 / 1,516 (37.20%)**. Another **801 methods**
are required to reach 1,365 (90%). Integration ports remain 38 / 662; overall
source mapping is 602 / 2,178 (27.64%).
Full replacement, configurable CLI merge formats and Windows remain pending.
