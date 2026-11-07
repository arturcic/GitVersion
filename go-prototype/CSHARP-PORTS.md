# C# calculation and configuration test ports

Source: immutable C# revision `14d222235b7dabb7033473528a76fd18478a9217`.
Implementation checkpoint: `b7074272e`.

This batch adds **38 mapped C# methods**, bringing the explicit inventory to
**44 of 2,178 declarations (2.02%)**. The remaining 2,134 declarations are pending.
These are static method declarations, not expanded NUnit discovery or a measure
of overall replacement compatibility.

## Retained behavior

| C# source class | Methods | Retained assertions |
| --- | ---: | --- |
| MainScenarios | 15 | Deployment modes, absent/bad tags, next-version above/equal/below tags, tagged tip versus later commits, and develop-only histories |
| ContinuousDeliveryTestScenarios | 10 | Fallback/configured versions and every intermediate feature merge, branch removal, cancelled release and completed release checkpoint |
| FallbackVersionStrategyScenarios | 2 | All four increment parameters for configured branch increments and commit-message increments |
| BranchWithoutCommitScenarios | 1 | All three prerelease tag parameters competing with a release branch version |
| VersionBumpingScenarios | 4 | Prerelease increment, minor/major messages, tag priority and resetting the version source |
| ConfigurationProviderTests | 6 | Overrides versus defaults, exact strategy selection, empty labels, large patch components and default source relationships |

There are **87 calculation checkpoints across 47 recipes**, plus **six direct
configuration cases**. Each calculation checkpoint retains the original
`FullSemVer` assertion and compares all 33 JSON fields/types against separately
captured C# output. The configuration cases assert public values directly in Go
and against the pinned C# effective YAML on each platform.

`testdata/csharp-port-mappings.json` records the source file/hash, method, parameter
case, Go test, recipe/checkpoints and adaptation notes. The inventory generator
checks these hashes against immutable Git blobs and refuses missing or unmapped
cases. No expected version is obtained from the Go implementation.

## Deliberate adaptations

- Builder/legacy flat configuration becomes explicit GitFlow/v1 or GitHubFlow/v1
  v7 YAML. A global deployment mode is expanded into explicit branch modes.
  This does not claim legacy v6 configuration loading or builder API compatibility.
- Tests that change configuration between assertions use independent fixtures
  retaining the same history at the corresponding checkpoint.
- Deterministic commit identities, timestamps and file contents replace generated
  values. Branch topology, tags, messages, deletion and original version assertions
  are retained. Initial `develop` repositories contain no artificial `main` ref.

## Defect found and fixed

`ShouldUseTheFallbackVersionOnDevelopWhenNoVersionsAreAvailable` failed in Go
because calculation required exactly one main/master branch. Non-Mainline
strategies now allow a repository without that ref and defer inheritance decisions
to the existing source finder. The original failure is retained in the local
first-run log. Four additional original develop next-version checkpoints pass.
`TestMissingMainDoesNotBypassSourceRequirements` verifies that an inherited orphan
still rejects and that unsupported Mainline-without-main topology stays explicit.

## Validation

Final platform results and hashes are recorded in `CSHARP-PORTS-EVIDENCE.json`.
The regular Go regression suite runs on macOS arm64 and Ubuntu 26.04.1 arm64.
Fresh independent process checks use `TestPlatformCSharpPortsParity` and
`TestPlatformCSharpConfigurationPorts`. The Ubuntu container has no network or
bind mounts; source/data and Linux test binaries are copied to its native filesystem.
The Go candidate remains CGO-disabled and uses go-git.

The previous strategy/configuration batch remains separate historical evidence in
`CONFIGURATION-EVIDENCE.json`. Its full fresh-process matrix was not rerun for this
batch; its frozen C# regressions are included in the regular suite on both systems.
The existing 89.7%/78.7% statement coverage measurements are from the previous
checkpoint, not a fresh measurement of this batch.

Windows, broader custom configuration, remaining source tests, CLI and integrations
remain open. CodeRabbit is skipped as requested. Everything stays local.
