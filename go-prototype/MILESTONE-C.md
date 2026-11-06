# Milestone C — independent Go calculation

Implemented locally on November 6, 2026 in the existing isolated `codex/go-prototype-milestone-a` worktree. No commits, pushes, publications or external reviews. CodeRabbit remains skipped at the user's request.

## Implementation

`gitversion-go calculate --repository PATH --output json` independently reads raw configuration and Git history and emits the pinned 33-field JSON contract. `--trace` emits a separate JSON diagnostic on stderr. The calculator uses go-git and checksum-pinned YAML v3; it has no runtime C#, libgit2, cgo, native Git command or reference-corpus dependency.

- `internal/semver`: strict versions, parsing, comparison and GitVersion-specific prerelease increments.
- `internal/config`: explicit frozen GitHubFlow/v1 settings, strict YAML validation, branch matching and C#-compatible sanitization.
- `internal/calculate`: tagged/configured/fallback candidates, bump messages, main/feature deployment behavior, independent semantic and counting sources.
- `internal/output`: all 33 JSON variables, including nullable numeric fields, source aliases, weights, assembly versions and UTC date formatting.
- `cmd/gitversion-go`: calculation and existing observation commands.

The frozen source remains `14d222235b7dabb7033473528a76fd18478a9217`, Release/net10.0 with runtime 10.0.14. The 25 original recipes, 41 portable expected outputs and baseline lock were not regenerated or altered.

## Acceptance contract

The original 41 checkpoints must match all 33 fields and JSON types. A separate live acceptance corpus adds 22 checkpoints with different version numbers, branch names, prerelease counters, explicit increments, competing/equal candidates, message precedence, Unicode labels/whitespace, configured versions with tag anchors, and a tag on advancing main not yet merged into feature.

The harness captures C# and executes the built Go candidate on independent repository copies. It checks process success, baseline integrity, unchanged repository contents, every output field and the immutable original reference. It records candidate SHA256 and recipe/lock digests. A relocated execution with an empty `PATH` confirms the command can run without reference tools; trace JSON is checked independently of stdout. No expected output is regenerated to accommodate Go.

## Requirement evidence

| Requirement | Evidence |
| --- | --- |
| Original full-JSON parity | `TestGoCalculationMatchesFrozenCorpus`: 41 checkpoints, exactly 33 fields, immutable expected JSON and unchanged repository state |
| Fresh C# / built Go parity | `TestPinnedGoCalculationParity`: 63 checkpoints, real isolated processes, retained provenance, relocated empty-PATH run |
| Raw independent YAML configuration | `TestFrozenConfigurationDefaultsAndOverrides`, `TestFrozenConfigurationRejectsUnsupportedInput`, `TestConfigurationFileDiscovery` |
| Strict semantic-version behavior | `TestSemanticVersionParsingAndFormatting`, `TestSemanticVersionIncrementAndOrder` |
| Separate semantic source and counting anchor | `TestCalculationConfiguredSourceKeepsTagAnchor`; original unrelated-prerelease case also compares all source fields |
| Unicode branch/message compatibility and alternative numbering | `TestBranchLabelsAndUTF16Sanitization`, `TestCalculationAlternativeAndUnicodeBumps`, plus corresponding live C# variants |
| Numeric/null/string output contract | `TestOutputVariablesAndSourceNullability`, `TestOutputStableAndMainDeployment` |
| Unsupported policies fail without a version | `TestCalculationRejectsUnsupportedPolicies`, `TestCalculationErrorForMissingConfig`, existing repository-state rejection tests |
| CLI JSON and trace separation | `TestCalculateCLISeparatesTraceAndRejectsUnsupportedConfig`, `TestUnsupportedCommandProducesNoJSON` |

## Review and scope

Three differences were found and fixed through comparison/review: dots in branch-derived labels are sanitized before substitution; an alternative higher core version resets prerelease numbering when the configured increment is None; .NET bump regex whitespace includes Unicode separators that Go's default `\s` excludes. Regression tests and independent live C# cases protect all three.

Supported configuration and explicit rejection boundaries are documented in `README.md`. Only the declared GitHubFlow subset is implemented. Mainline, GitFlow release/hotfix policies, arbitrary configuration/regex/format strings, reset directives, CI normalization/output, caches, MSBuild and broad repository-state support remain out of scope. macOS arm64 is the only validated platform so far. Milestone D covers generated histories, real-history replay, cross-platform validation and benchmarks; these gates are not claimed by Milestone C.

## Final validation

The combined `CGO_ENABLED=0` run passed **347 tests/subtests across nine packages, with zero failures or skips**. This includes the original reference capture/replay and deliberate wrong-candidate/tamper checks, repository observation parity, and independent Go version calculation. Build, vet, module verification and formatting checks passed. The compiled Go CLI closure has no CgoFiles.

- Final test events: `.local/milestone-c-tests.jsonl`.
- Calculation comparison: `.local/calculation-evidence/run-3597986098/` — `provenance.json` is complete, 63 passed, zero failed; 41 original plus 22 additional checkpoints, all 33 fields (2,079 field comparisons).
- Go executable SHA256: `8781242595b1447f006578727590a1cd078cf8b670e93e666af615a2c65805f6`.
- Repository observation comparison: `.local/observation-evidence/run-100207284/` — 47 snapshots and 2,233 ordered-walk/merge-base queries.
- Fresh original harness acceptance: `.local/integration-evidence/run-3404007318/`.
- Baseline lock SHA256 remains `cfbc6c9fb49026cb16087eb1c53c8b8ec69b1da448aa1fdaf1f7b44c8791d79a`.

Failed intermediate runs are retained separately, including the dotted-label mismatch. Acceptance checks did not regenerate original expected outputs. Everything remains local and uncommitted; the original checkout is clean.
