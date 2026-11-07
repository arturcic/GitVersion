# C# parsing, formatting and regex ports

Implementation checkpoint: `57b651354`.
Pinned C# source: `14d222235b7dabb7033473528a76fd18478a9217`.

This batch ports **42 complete method declarations and 171 original cases**:

- All 35 methods in `Core/RegexPatternTests.cs`, including all 122 literal cases.
  Original match success, whole-match values, ordered match collections and
  numbered capture assertions are preserved.
- The remaining seven methods in `VersionCalculation/SemanticVersionTests.cs`,
  including all 49 parsing/formatting cases. Assertions cover every original
  component, nullable prerelease number and metadata field, plus all expected
  informational/default/s/j/f/i strings and invalid-format rejection.

Together with the preceding increment batch, all 21 attributed methods in the
pinned `SemanticVersionTests.cs` now have complete source mappings. This does not
claim all semantic-version behavior is covered: other classes have additional
comparison, prerelease and metadata tests still pending.

## Implementation

`internal/patterns` uses the pure-Go `regexp2/v2` engine for .NET-style patterns.
The extractor reads both immutable test source and production pattern constants,
including their declared options. Generated patterns are embedded production
inputs; expected test values are extracted independently from NUnit attributes.
It fails on unknown syntax/options or changed inventories. Both extractions are
deterministic. Source, case-file hashes and exact case IDs bind every mapping.

The wrapper matches UTF-16 code units and reconstructs captures, preserving
supplementary-character behavior for the tested contracts. Match execution has a
two-second bound. Culture-sensitive .NET casing differs from Go Unicode simple
case folding in the underlying engine; arbitrary custom-regex parity is still
open. This batch does not implement the C# regex-cache API or replace every
existing calculator regex use.

`semver.ParseDetailed` adds the general version-model contract: strict and loose
parsing, custom prefixes, 64-bit numeric components, prerelease normalization and
nullable build metadata. `Version.Format` and `BuildMetadata.Format` reproduce the
mapped format rules. The calculator's existing bounded `Parse` entry point is
unchanged; CLI acceptance of metadata, loose versions and wider arithmetic still
requires separate end-to-end work.

## Verification matrix

| Requirement | Evidence |
| --- | --- |
| Original built-in regex cases | `TestCSharpRegexPatterns` — 35 methods, 122 cases |
| Original version parsing and formats | `TestCSharpSemanticVersionParsingAndFormats` — seven methods, 49 cases |
| UTF-16 capture and failure behavior | `TestUTF16PatternCapturesAndFailures` — supplementary captures/literals, empty matches, invalid and unknown patterns |
| Parsing boundaries and nullable metadata | `TestDetailedVersionParsingBoundaries` — numeric zero/max/overflow, invalid modes/prefixes, sanitization, metadata without count |
| Calculation and configuration regression | Full regular/frozen suite; fresh `TestPlatformCSharpPortsParity` and `TestPlatformCSharpConfigurationPorts` on each OS |

Original unit expected values come from pinned C# source. The original NUnit
runner is not executed by this port harness. Fresh C#/Go process comparisons
cover 87 calculation checkpoints (all 33 JSON fields/types) and six effective
configuration value cases. The older complete strategy matrix is historical
evidence, not a fresh full-matrix run in this batch.

Both macOS arm64 and Ubuntu 26.04.1 arm64 passed **4,659 regular tests/subtests in
10 packages**, with no failures or skips, plus the fresh process comparisons above.
`CSHARP-MODEL-PATTERNS-EVIDENCE.json` binds reports, source inputs, candidates,
logs and exported Linux archives to the implementation checkpoint. The isolated
container ran without networking or host mounts and was removed after its
evidence was exported.

Verified unit/component ports now total **62 / 1,516 (4.09%)**; integration ports
remain 38 / 662, for 100 / 2,178 mapped declarations overall. The 90% goal needs
1,365 verified unit methods: **1,303 more remain**. All work stays local; Windows
stays last.
