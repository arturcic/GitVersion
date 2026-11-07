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

Full platform gates are running. Methods remain unverified in the inventory until
both macOS and Ubuntu execution complete. Windows stays pending. Earlier batch
reports retain their historical results; this report supersedes their checkout
blocker once the new platform evidence is recorded.
