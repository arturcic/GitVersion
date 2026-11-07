# Version sources and ignore configuration

C# baseline: `14d222235b7dabb7033473528a76fd18478a9217`.

All 19 VersionSourceTests methods / 27 cases retain their original FullSemVer,
semantic source/version/increment, counting source/distance and legacy source
assertions. Deterministic go-git histories preserve parent order and branch/tag
operations. Original SHA variables bind to checkpoint identities. TrunkBased
builder settings are projected to explicit Mainline settings for these cases.
Operator assertions run through the production Mainline base/operator model;
the original mocked strategy supplies candidates to the shared strategy callback.

C# model-null legacy SHA maps to the empty Go trace string. That model assertion
is checked separately from the legacy JSON formatter's empty-string contract.
The source harness does not execute NUnit and does not generate expected versions
from the Go calculator.

`calculation.ignore` now accepts `sha`, `commits-before` and `paths`. SHA matching
uses case-insensitive prefixes; date filtering excludes earlier timestamps; path
filtering excludes a commit only when every changed path matches. Empty configured
lists do not create active filters. Rules apply to current-commit selection,
tag/source candidates, bump messages, Mainline nodes and commit counting. Graph
walks retain their original ancestry before filtering. Branch/tag name ignore
patterns remain explicitly unsupported in this prototype.

| Requirement | Evidence |
| --- | --- |
| Every original source/count/operator assertion | `TestCSharpVersionSources` |
| Typed ignore lists, dates, deduplication and validation diagnostics | `TestIgnoreConfigurationParsing` |
| Full version output across both workflows, Mainline, and ignore combinations | `TestPlatformRepositoryCompatibilityParity` |

The fresh process matrix adds 32 histories: two workflows × normal/Mainline ×
eight ignore settings. It covers full/prefix SHA exclusions, ignored bump messages,
date boundaries, ignored paths/current commits and empty rules. Configuration is
committed after the referenced SHA exists, using go-git and fixed metadata.
The 57-case repository matrix includes the previous 25 compatibility checkpoints.

Implementation checkpoint: `3a24a027a`. macOS arm64 and Ubuntu 26.04.1 arm64
each passed **37,789 regular tests/subtests in 11 packages**, with no failures or
skips. All 27 source cases and configuration checks ran on both OS.

Fresh executable comparisons passed 57 repository/ignore JSON cases, 88 global-mode
JSON cases, 87 prior calculation JSON cases and six configuration checks per OS.
All 33 real-repository JSON fields/types also match: **7.0.0-alpha.1+739**.
`CSHARP-VERSION-SOURCE-EVIDENCE.json` records source/binary/report/corpus hashes,
verified against the implementation checkpoint and the actual Linux input bundle.
Complete Linux inputs, fixtures, process results and logs are archived locally.
The container used no network or host mounts. Source extraction is deterministic;
`go vet ./...` and whitespace checks pass. Windows remains pending.

Verified unit/component total: **595 / 1,516 (39.25%)**, with **770 more methods**
required to reach 1,365 (90%). Integration ports remain 38 / 662; overall mapping
is 633 / 2,178 (29.06%). These percentages measure source-method ports, not line
coverage or full replacement compatibility. Branch/tag-name ignore expressions,
legacy configuration loading and remaining CLI/integration contracts stay pending.

The later [ignore configuration checkpoint](CSHARP-IGNORE-CONFIGURATION-PORTS.md)
supersedes the branch/tag name filter limit above and adds selected v6 loading
for implemented fields. Other legacy fields and CLI coverage remain pending.
