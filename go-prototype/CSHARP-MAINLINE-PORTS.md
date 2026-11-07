# Mainline source ports and repository compatibility

C# baseline: `14d222235b7dabb7033473528a76fd18478a9217`.

The source extractor ports all 380 methods and 32,064 original parameter cases
from 79 `MainlineScenariosWithAGitHubFlow+*.cs` files. It preserves each file's
actual workflow builder, including GitFlow builders, original expected FullSemVer,
branch/tag positions, commit messages, parent order and requested branch deletion.
Fixed identities/timestamps and independent text files make histories deterministic.
Null builder labels remain distinct from empty labels. Parsed configurations enter
the same production calculator without changing a fixture's configuration file.
The harness does not execute NUnit; expected values come from immutable C# literals.

| Requirement | Evidence |
| --- | --- |
| Original Mainline versions and all parameter combinations | `TestCSharpMainlineScenarios` |
| Inherited orphan branches still require a source | `TestMissingMainDoesNotBypassSourceRequirements` |
| Worktree configuration marker matches the pinned common-config reader | `TestWorktreeConfigMarkerUsesPinnedCommonConfiguration` |
| Other unsupported extensions still fail | `TestWorktreeConfigAdapterPreservesUnknownExtensionRejection` |
| Ignored child under a reincluded parent remains clean; tracked edits remain visible | `TestIgnoredChildUnderReincludedParentIsClean` |
| Ignore anchors, classes, escapes, case and directory boundaries | `TestManagedIgnoreEntryMatching`, `TestIgnoredParentCannotBeReincludedFromBelow` |
| Remote symbolic refs resolve and private refs are ignored | `TestBranchReferencesResolveAndPrivateReferencesAreIgnored` |
| Config discovery matches case-insensitive C# filename priority | `TestConfigurationFileDiscovery` |
| Issue-number branch names are not strict semantic versions | `TestBranchVersionExtractionRequiresStrictVersion` |
| Unknown branch names use preset defaults | `TestUnknownBranchUsesWorkflowDefaults` |
| Fresh C# process parity for the repository compatibility combinations | `TestPlatformRepositoryCompatibilityParity` |

The repository adapter removes only the worktreeconfig marker from the parsed
in-memory configuration passed to go-git. It does not rewrite Git settings. The
pinned C# managed backend reads common configuration and ignores config.worktree;
this is compatibility with that baseline, not general Git overlay semantics.
Calculator traversal reads commit metadata without unnecessary tree diffs.

Implementation checkpoint: `d7f2796a6`. macOS arm64 and Ubuntu 26.04.1 arm64
each passed **37,617 regular tests/subtests in 11 packages**, with no failures or
skips. This includes every one of the 32,064 original Mainline parameter cases.
Each OS also passed 17 fresh repository compatibility JSON comparisons, 88 global
mode comparisons, 87 prior calculation comparisons and six configuration checks.

The actual GitVersion checkout at `9b63978c52ee9f634c46c6795d867920ae97f528`
returns **7.0.0-alpha.1+739** in both C# and Go. All 33 JSON fields and types match
on macOS and Ubuntu. The Linux snapshot contains the same tracked files, Git
objects/references and configuration, excluding ignored host build output.
Separate fresh fixtures cover ignored build-file behavior. The original checkout,
Git configuration and version configuration were unchanged. The Linux container
ran without network access or host mounts. Full logs, process outputs and fixture
archives are retained locally; hashes/provenance are in `CSHARP-MAINLINE-EVIDENCE.json`.

The source extractor reproduces all 81 generated corpus/index/mapping files
unchanged. Source input hashes are verified against the implementation checkpoint
and the actual Linux input archive. `go vet ./...` and whitespace checks pass.
Original NUnit is not executed by the source-port harness. Windows remains pending.
This batch supersedes the historical worktreeconfig blocker in earlier reports.

Verified unit/component ports now total **549 / 1,516 (36.21%)**, with **816 more**
needed to reach 1,365 (90%). Integration ports remain 38 / 662; overall mapping is
587 / 2,178 (26.95%). These are source-method port percentages, not line coverage
or proof of full product replacement. CLI/integrations and remaining configuration,
repository and strategy contracts continue under the active goal.
