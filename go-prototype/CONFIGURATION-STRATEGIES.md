# Configuration and strategy expansion

This local checkpoint extends the pinned C# replacement experiment with:

- Every subset of Fallback, ConfiguredNextVersion, MergeMessage, TaggedCommit,
  TrackReleaseBranches, VersionInBranchName and Mainline; empty/None, duplicate
  and reordered selections are explicit cases.
- Branch main/release classification, release tracking, merge-target and
  merge-message tracking, and both merged-branch increment prevention flags.
- Prerelease next-version matching, numeric prerelease retention, and strategy
  competition; stable winners exclude prerelease counting sources.
- Competing source branches and Mainline synthetic inheritance transitions.
- Anchored branch-part version extraction, including accidental embedded numbers.

The new frozen corpus contains **1,966 outcomes: 1,885 complete JSON outputs and
81 exact-fixture process rejections**. These are 768 strategy cases, 800 branch
configuration/prerelease cases, 14 edges, and 384 additional before/after merge checks. Every output field and JSON type is
checked. Rejections are separately counted, pinned to fixture digests, require
exit 1, empty stdout and unchanged inputs, and cannot accept timeouts or other
exit codes. Diagnostic wording remains a CLI compatibility gate.

The 800-case matrix exposed two missing behaviors: competing configuration
origins and Mainline dummy transitions at a shared branch point. Both were fixed
from pinned source and verified against unchanged C# captures. No existing
reference output was rewritten.

Six original C# methods / seven parameter cases are linked and ported in
`CSHARP-TEST-REUSE.md` and `testdata/csharp-test-inventory.json`. The inventory
contains 2,178 attributed declarations, not expanded/discovered test counts.

## Final validation

- `TestStrategyFrozenReference`: 688 JSON matches + 80 no-candidate rejections.
- `TestConfigurationFrozenReference`: 800 JSON matches.
- `TestConfigurationEdgeFrozenReference`: 13 JSON matches + one rejection,
  retaining seven original C# FullSemVer assertions.
- `TestStrategySelection`, `TestBranchControlOverrides`, and
  `TestFrozenRejectionContract`: parser choices, explicit false inheritance,
  invalid types and rejection negative controls.
- `TestMergeControlFrozenReference`: 384 before/after merge comparisons with
  independent source/destination prevention settings. These ensure a later tag
  cannot hide a merge-control defect.
- `TestConfigurationCorpusValidation`: **1,232 distinct raw v7 configurations**
  validated by the pinned C# CLI, with effective YAML captured. This is a C#
  configuration audit; Go CLI configuration commands remain future work.
- Regular suite plus refreshed harness package: **3,159 distinct tests/subtests**,
  nine packages, no failures/skips. `TestEffectiveConfigurationRejectsMutation`
  covers mutation and missing state evidence during effective-config capture.
- On **each** of macOS arm64 and Ubuntu 26.04.1 arm64: **3,568 exact JSON
  matches + 82 separate expected rejections**, zero failed cases. This includes
  all 684 earlier curated outcomes, the 1,000 generated workflow outcomes, and
  1,966 new outcomes. `TestPlatformStrategyParity`,
  `TestPlatformConfigurationParity`, `TestPlatformConfigurationEdgeParity` and
  `TestPlatformMergeControlParity` are the new process gates.
- `COVERAGE.json`: **89.7%** implemented CLI/core statement coverage; **78.7%**
  whole-module coverage including fixtures and harness. This uses regular
  per-package Go coverage and a refreshed harness profile; build-tagged process
  runs are excluded. These percentages do not measure missing functionality or
  migration of the C# test suite.
- All-tag vet, module checksums, formatting and whitespace checks pass. The
  candidate dependency graph has no libgit2, git2go or runtime/cgo.
- Both Ubuntu containers used the pinned image, native filesystems, no bind
  mounts and networking disabled. Complete execution evidence is retained in
  checksum-recorded compressed archives; report indexes are extracted locally.
  Both temporary containers were removed after verification.

`CONFIGURATION-EVIDENCE.json` records all 20 platform reports, candidate hashes,
source input hashes, baseline identity, configuration audit and archive hashes.
The implementation checkpoint is `f41a4f190`. Candidate binaries were built before
that commit and embed the previous VCS revision plus modified=true; their actual
production input bytes were verified against `f41a4f190` and recorded explicitly.
No candidate implementation code changed after the binaries were built. The subsequent
checkpoint contains tests, evidence and dashboard updates.

The initial macOS platform harness preflight could not parse the runtime path
containing spaces; it ran no calculation comparisons. The harness now reuses the
baseline parser, `TestRuntimeDirectoryPreservesSpaces` passes, and the complete
fresh macOS run passed. No failed version result was rewritten or omitted.

## Remaining compatibility gates

This is not full configuration compatibility. Custom branch/source policies,
.NET-compatible regular-expression overrides, ignore rules, loose SemVer parsing,
YAML default/null/anchor semantics, global defaults, output schemes/templates and
other format/environment settings still require independent reference cases.
CLI/repository-state/CI/MSBuild contracts and full C# test-suite migration remain
open. Windows stays last. No pushes, pull requests or external review were made.
