# Full replacement compatibility contract

The target is **replacement of the established C# CLI and in-scope integrations**, pinned to `src/` at `14d222235b7dabb7033473528a76fd18478a9217`. MSBuild, experimental `new-cli/`, SSH, and legacy CLI arguments are outside the active scope. Prioritize macOS and Ubuntu 26.04; validate Windows last. Keep all implementation, build artifacts and evidence local. This target is not achieved yet.

Compatibility is an inventory of observable behaviors, not a percentage inferred from passing tests. A group closes only when positive, negative, boundary and interaction cases match the pinned reference on both priority platforms. Unsupported cases remain open even when the Go executable rejects them cleanly. Intentional differences require an explicit decision; do not silently call them compatible.

| Area | Current state | Remaining acceptance work |
| --- | --- | --- |
| GitHub Flow main/feature calculation | Verified declared subset | Reset directives verified; additional configuration interactions, unusual histories and version bounds |
| Deployment modes and branch controls | First E batch verified | Remaining per-family edge interactions and custom/default/null policies |
| GitFlow and branch families | Declared default-family subset verified on macOS/Ubuntu | Competing built-in configuration sources now covered; full custom/fallback branch policies and further merge interactions remain open; see CONFIGURATION-STRATEGIES.md |
| Mainline | Declared strategy subset verified on macOS/Ubuntu | All 128 strategy subsets covered on six histories; further configuration/topology interactions and exact rejection diagnostics remain open |
| Configuration | Strategy/control subset verified; 1,232 C# configuration audits | All v7 sections/defaults, source branches, custom regex/formatting, ignore paths/SHAs/dates, override precedence, lookup and diagnostics |
| Semantic versions and output variables | 33 fields verified within subset | Loose format, build metadata, additional prerelease next-version boundaries, full bounds, custom/assembly/date formats |
| Repository layouts and state | Complete clean attached local repositories | Worktrees, detached/shallow/bare/dirty repositories, remote refs, missing objects, normalization and supported failure behavior |
| CLI | Root POSIX CLI and configuration commands verified in selected cases. [Logging evidence](CSHARP-CLI-LOGGING-EVIDENCE.json) covers verbosity and sink routing on macOS and Ubuntu 26. | Nested version-strategy diagnostics, remote and CI traces, cache details, exception traces, and broader option interactions |
| Build-server adapters | Partial | Broader provider precedence, failure paths, repository side effects, and downstream CI smoke tests |
| MSBuild | Out of scope | Existing prototype task results are historical evidence. |
| Distribution and embedding | Open | Local native binaries, package layout/entry points, .NET task/API compatibility boundaries, reproducible packaging and consumer smoke tests |
| macOS | Verified existing corpus on arm64 | Each new group; architecture matrix and filesystem edge cases |
| Ubuntu 26.04 | Verified 3,568 JSON comparisons + 82 rejection contracts on arm64 | Each new group; architecture matrix and filesystem edge cases |
| Windows | Explicitly last; not executed | Portable handoff, platform-specific behavior, all declared groups and integrations |

## Source-backed inventory

The pinned source is extracted under `.local/source/`; do not use a moving branch as the reference.

- `src/GitVersion.Configuration/GitVersionConfiguration.cs`, `BranchConfiguration.cs`, `ConfigurationDocumentMapper.cs`, configuration builders and the generated v7 schema define the configuration surface. Calculation/output/normalization settings must retain their v7 locations.
- `src/GitVersion.Core/VersionCalculation/` defines version strategies, deployment and increment/reset semantics. Test merge ordering and configuration-source inheritance, not only rendered version strings.
- `src/GitVersion.App/` and the captured executable help/schema define CLI contracts. [CLI-COMPATIBILITY.md](CLI-COMPATIBILITY.md) enumerates all 42 captured command/option entries. The current prototype CLI is not yet a command-line drop-in replacement.
- `src/GitVersion.BuildAgents/Agents/` contains GitHub Actions, GitLab CI, Azure Pipelines, TeamCity, Jenkins, AppVeyor, Bitbucket Pipelines, Buildkite, CodeBuild, Continua CI, Drone, EnvRun, MyGet, Space Automation and Travis CI adapters. AppVeyorJsonContext is a helper, not a separate provider.
- `src/GitVersion.MsBuild/Tasks/`, `.props`/`.targets`, and public API files define MSBuild consumer contracts. A pure Go executable cannot directly be loaded as a managed MSBuild task; preserve the consumer interface through an explicitly tested bridge if needed. The core remains Go/go-git with no libgit2 dependency.

## Execution order

1. Ubuntu 26.04 workflow gates are verified; retain image/runtime/package provenance for every new group.
2. Expand calculation/configuration in reference-first batches. Reset-message and declared GitFlow/Mainline semantics are verified; strategy/control combinations and competing built-in source branches have a subsequent batch. Custom regex/source policies, ignore rules, YAML defaults and output formats remain open.
3. Implement the pinned CLI surface and all output formats, diagnostic/exit behavior and filesystem side effects.
4. Validate build-server adapters and downstream CLI consumers with isolated local stand-ins. Real CI publication or network operations require separate authorization.
5. Run the complete declared matrix on macOS and Ubuntu, including generated histories, public snapshots, repeatability, consumer builds and performance regressions.
6. Prepare a self-contained Windows handoff and run the final platform matrix in the VMware guest when requested.

Windows handoff must contain source including uncommitted files, the pinned baseline specification/source identity, immutable fixture/golden corpora, dependency checksums, exact commands and current mismatch inventory. macOS binaries and machine-specific baseline locks cannot be treated as Windows builds. A new session must build and record its own platform provenance; copying a chat is not a substitute for copying the local work.

The original C# oracle may still package its historical fallback, but every oracle invocation selects the managed backend. The Go core and its Git access remain pure Go. No Windows VM is started by this work.
