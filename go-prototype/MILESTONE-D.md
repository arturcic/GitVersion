# Milestone D — broader compatibility evidence

Local work on November 6, 2026. No commits, pushes, publications or CodeRabbit review. **The full milestone exit gate is not met: Windows execution remains unverified.** The local implementation and available execution checks are complete: macOS and Linux, generated parity, public replay, benchmarks and the combined suite passed.

## Generated histories and findings

`TestGeneratedCalculationParity` passed **1,000/1,000** independently generated checkpoints against fresh pinned C# processes, comparing all 33 fields (33,000 field comparisons). Seed `20261106` produced 8,128 ordinary commits, 952 merge commits, 3,194 tags (1,613 annotated), 1,622 branch creations, and 698 final feature-branch checkpoints. Graph shape, configuration, messages and equal/skewed timestamps vary. The exact recipes and snapshot identities are retained; a seed is not the only replay contract.

The first 64-case pilot found 12 differences; after fixing current-commit selection, 64/64 passed. The first full run then found 23 feature-inheritance differences (977/1,000 passed). Both failed runs remain available. Three compatibility fixes resulted from D:

- The pinned managed C# backend selects the first time-ordered branch-log commit. With clock skew, that can be an ancestor rather than physical HEAD. Go now preserves that behavior.
- The final alternative-version adjustment uses the branch supplying inherited configuration. For this feature-branch subset, that is main; using the feature branch produced the 23 differences.
- go-git's protected worktree filesystem cannot read `.git/info/exclude` through its normal ignore scan. The adapter now reads it explicitly, preserves `.gitignore` precedence, and never hides tracked or staged changes.

All 35 generated failures were reduced from **667 total recipe steps to 179**. Reduction removes steps until no single deletion retains the relevant difference against an isolated reconstruction of the old behavior. It is not a globally minimal graph proof. Every resulting recipe was independently recalculated with pinned C#, and the fixed Go calculator matched every field. These new expected outputs are in `testdata/generated-regressions.json`; the original 41-case golden corpus was not regenerated.

## Public histories and repeatability

`testdata/public-history.json` pins six checkpoints from **GitTools/actions** and **GitTools/build-images**, including roots, a tagged merge, intermediate history and a feature projection. `PublicSnapshot` copies complete reachable Git objects from existing local stores using go-git; it does not clone, fetch, rewrite commits, use alternates or modify the source repository.

These are **real histories with explicit experimental inputs**: the manifest selects tags, projects main/feature refs, and supplies the frozen GitHub Flow configuration as an ignored working-tree overlay. This does not establish compatibility with either project's actual release policy. Existing project configuration is rejected rather than overwritten.

All six cases match. Three executions per calculator/case (36 executions) also preserve every field after relocation, locale/time-zone changes and removal of tools from PATH. Repository state is checked before/after every process. Benchmark samples are additional verified executions, not additional distinct compatibility cases.

## Platform matrix

| Platform | Execution evidence | Result |
| --- | --- | --- |
| macOS arm64 | Full A–C suite, 1,000 generated cases, six public cases and 35 minimized regressions | Passing |
| Linux arm64 | Same 63 curated cases, fresh C# and Go, original 41 outputs also compared to the portable golden | 63/63 passing |
| Windows amd64 | Calculator and platform-test executable cross-compiled with CGO disabled | Execution unverified; gate open |

Linux ran in a local container with networking disabled, using the exact C# published assemblies and runtime 10.0.14. Its host/runtime checksums are recorded separately; the macOS baseline lock is never overwritten or misrepresented as a Linux runtime lock. Container image: `mcr.microsoft.com/dotnet/runtime@sha256:4f1f30cf0bb4ab0c8b2daa84613051683456f8df4a9bf7c812e20d74962992ac`.

`scripts/run-platform.ps1` is prepared for a Windows host with .NET runtime 10.0.14 and the local module/baseline files. Cross-compilation is not a parity pass. No accessible Windows runner was established locally; no remote CI was started.

Linux execution, from the module directory after cross-building the two binaries:

```bash
docker run --rm --network none \
  --mount "type=bind,source=$PWD,target=/work" --workdir /work/cmd/parity \
  -e PARITY_DOTNET=/usr/share/dotnet/dotnet \
  -e PARITY_CANDIDATE=/work/.local/platform-bin/linux-arm64/gitversion-go \
  --entrypoint /work/.local/platform-bin/linux-arm64/parity.test \
  mcr.microsoft.com/dotnet/runtime@sha256:4f1f30cf0bb4ab0c8b2daa84613051683456f8df4a9bf7c812e20d74962992ac \
  -test.run '^TestPlatformCuratedParity$' -test.v -test.timeout 15m
```

## Benchmarks

Final measurements were collected on macOS arm64 after the other experiment/test jobs finished. The benchmark runs only already-passing cases, checks every sample's output and repository state, alternates calculator launch order, disables C# result caching, and uses 20 samples per calculator for two histories. Times include process startup but exclude fixture copying/state hashing. Filesystem caches are warm. Peak RSS is measured through process resource usage (bytes on macOS; converted from KiB on Linux). Raw samples are retained.

| Public history | Go median / p95 | C# median / p95 | Go / C# median peak RSS |
| --- | --- | --- | --- |
| actions root | 10.9 / 11.5 ms | 395.1 / 445.2 ms | 13.4 / 91.5 MB |
| actions intermediate | 91.1 / 106.1 ms | 415.3 / 433.2 ms | 20.1 / 95.9 MB |

Sizes (decimal MB, uncompressed): Go single executable **11.3 MB**; published C# directory **28.4 MB**, plus **86.2 MB** of required runtime files. The runtime-directory size excludes the dotnet host; this is not a complete installer-size comparison. Both executables are the recorded local builds, without introducing additional stripping or optimization for this comparison.

No hard performance threshold was specified before this experiment; measurements inform the next decision rather than inventing a success threshold afterwards. Tiny historical repositories do not predict large-repository performance. Distribution figures distinguish the Go executable, published framework-dependent C# directory, and required .NET runtime. No NativeAOT/trimmed C# comparison is claimed. Child Git process counts are not instrumented; empty-PATH runs and offline Linux execution demonstrate successful operation without Git tools being supplied.

## Requirement evidence

| Requirement | Evidence |
| --- | --- |
| Deterministic generated inputs and count bounds | `TestGeneratedRecipesDeterministic`, `TestGeneratedRecipeBounds` |
| 1,000 exact generated comparisons | `TestGeneratedCalculationParity` |
| Replayable reduced failures | `TestRecipeReductionPreservesFailureAndInput`, `TestRecipeReductionNoFailureAndErrors`, `TestMinimizedGeneratedRegressions` |
| Clock-skew compatibility | `TestCalculationSelectsLatestCommitByReferenceOrdering` |
| Inherited branch determines alternative version | `TestInheritedBranchControlsAlternativeVersion` |
| Ignore rules retain tracked/staged dirty detection | `TestInfoExcludePrecedenceAndTrackedChanges` |
| Public object identity and isolated configuration | `TestPublicSnapshotPreservesObjectsAndIsolatesOverlay`, `TestPublicSnapshotRejectsExistingProjectConfiguration` |
| Public full-JSON parity, repeat/path/environment checks, benchmark output correctness | `TestPublicHistoryReplayAndRepeatability` |
| Curated cross-platform parity and portable fixture identities | `TestPlatformCuratedParity` (macOS via existing acceptance; Linux via platform runner; Windows unrun) |

## Evidence and remaining scope

- Generated final run: `.local/expansion-evidence/run-2544419238/` — complete, 1,000 passed, zero failures.
- Retained failed runs: `.local/expansion-evidence/run-2091460897/` (pilot), `run-1071288346/` (full run before inheritance correction).
- Linux final run: `.local/platform-evidence/run-3538339774/` — complete, 63 passed, zero failures.
- Go macOS executable SHA256: `dc82368b924a7aebe8af7bce527236a22d86a8e18c975d6f32f7ed7fafc50db8`.
- Baseline lock remains `cfbc6c9fb49026cb16087eb1c53c8b8ec69b1da448aa1fdaf1f7b44c8791d79a`.
- Final combined suite: `.local/milestone-d-tests-final.jsonl` — **398 tests/subtests across nine packages passed**, zero failures/skips. This excludes separately executed expansion/platform cases; it is not a sum inflated by repeated runs. Two final assertion-only strengthenings also passed their focused fixture tests.
- Fresh macOS curated parity: `.local/calculation-evidence/run-1843753012/` — 63/63.
- Fresh repository observation parity: `.local/observation-evidence/run-2785129179/` — 47 snapshots and 2,233 queries.
- Fresh harness negative/tamper acceptance: `.local/integration-evidence/run-2961402038/`.
- Public replay and final benchmark: `.local/public-evidence/run-786589188/` — complete; `benchmark-samples.json` retains all 80 timed executions, `benchmark-summary.json` contains median/p95/memory/size results. Every sample passed full-JSON comparison.
- Build, vet (including integration/expansion/platform test code), module verification, formatting and whitespace checks passed. Compiled CLI dependency closure has no CgoFiles. Linux and Windows candidates were cross-built with CGO disabled; Windows was not executed.

The tested GitHub Flow subset has stronger compatibility evidence, not a percentage of total GitVersion functionality. GitFlow/Mainline, broader configuration, CI normalization, build-server output, MSBuild, shallow/dirty/bare/detached repositories and other integrations remain outside declared support. At D completion, E had not started; subsequent expansion is documented in [MILESTONE-E.md](MILESTONE-E.md). Windows execution is required before declaring the original three-platform D gate complete.
