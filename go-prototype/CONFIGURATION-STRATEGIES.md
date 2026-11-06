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

The new frozen corpus contains **1,582 outcomes: 1,501 complete JSON outputs and
81 exact-fixture process rejections**. These are 768 strategy cases, 800 branch
configuration/prerelease cases, and 14 edges. Every output field and JSON type is
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

## Checkpoint validation

- `TestStrategyFrozenReference`: 688 JSON matches + 80 no-candidate rejections.
- `TestConfigurationFrozenReference`: 800 JSON matches.
- `TestConfigurationEdgeFrozenReference`: 13 JSON matches + one rejection,
  retaining seven original C# FullSemVer assertions.
- `TestStrategySelection`, `TestBranchControlOverrides`, and
  `TestFrozenRejectionContract`: parser choices, explicit false inheritance,
  invalid types and rejection negative controls.
- Full regular Go suite: 2,772 tests/subtests, nine packages, no failures/skips.
  A subsequent focused harness test covers runtime installation paths with spaces.
- Fresh macOS and Ubuntu 26 process comparison is in progress at this checkpoint.
  The initial macOS harness preflight rejected a runtime path containing spaces;
  the harness now reuses the baseline parser and has a regression assertion.

## Remaining compatibility gates

This is not full configuration compatibility. Custom branch/source policies,
.NET-compatible regular-expression overrides, ignore rules, loose SemVer parsing,
YAML default/null/anchor semantics, global defaults, output schemes/templates and
other format/environment settings still require independent reference cases.
CLI/repository-state/CI/MSBuild contracts and full C# test-suite migration remain
open. Windows stays last. No pushes, pull requests or external review were made.
