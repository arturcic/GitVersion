# GitFlow and Mainline parity checkpoint

This local Milestone E expansion targets the established C# `src/` implementation at `14d222235b7dabb7033473528a76fd18478a9217`, using its managed Git backend. The candidate calculates independently in Go with go-git and `CGO_ENABLED=0`.

## Implemented scope

- `GitFlow/v1` default main/master, develop, feature, release, hotfix, support and pull-request branch families; GitHub Flow release/pull-request families alongside its existing main/feature support.
- Preset branch inheritance, branch-specific labels/weights, release names and merge messages, tracking release branches and merge-target tags. Unversioned release branches can derive a stable target from their own matching prerelease tags; tags inherited from main or merged side branches are excluded from that fallback.
- Explicit `calculation.strategies: [Mainline]`, optionally with `ConfiguredNextVersion`. A configured floor is ignored when that strategy is not selected, matching C#.
- Mainline commit-by-commit trunk increments, branch accumulation, recognized/unrecognized merges, nested merges, tagged boundaries, default reset directives, semantic provenance and independent commit-count anchors.
- Existing deployment modes and accepted per-branch increment/message/label/weight/tag-prevention settings extend to the new preset families. Unknown settings continue to fail explicitly.

This is a verified workflow subset, not completion of the full replacement contract. Custom source-branch lists/regex, the complete strategy-combination matrix, remaining track/prevent controls, arbitrary labels/formatting, multiple competing configuration sources, normalization and other repository layouts still require their own acceptance cases. Mainline merges between two main-classified branches (including main/support) remain rejected by the pinned strategy; supporting a support branch does not imply successful versions for those merges. CLI syntax/diagnostic equivalence, CI adapters, MSBuild and distribution remain separate open work. Windows stays last.

## Reference-first evidence

The original goldens remain unchanged. `workflow-reference.json` was captured before calculator changes: 29 raw histories / 166 checkpoints. `workflow-edge-reference.json` adds 13 checkpoints covering release-tag ownership and configured Mainline floors. Both freeze all 33 JSON fields, including types, nulls and source/count provenance; tests also verify fixture identities and unchanged repository contents.

The deterministic generator uses seed `20261106`, combining GitFlow, GitHub Flow Mainline and GitFlow Mainline with the existing DAG/policy generator. It includes equal/skewed timestamps, annotated tags, repeated/nested merges, reset messages and explicit policy controls.

The first 64-case pilot exposed a child-operand provenance error. The first 1,000-case run exposed 22 JSON mismatches and one C# rejection. Fixes preserve the distinctions between:

1. A merge child's operand and its pending alternative version.
2. Topological child walks, timestamp-ordered branch walks and the default-order forward-merge query.
3. A tag's original commit and its merge-target lookup group when ordering candidates.
4. The tag view inherited by child traversal and views replaced at branch transitions.

The 23 failing successful-reference histories (22 from the large run plus the pilot case) are retained in `workflow-generated-regressions.json`, with independent C# output. These are retained full histories, not claimed to be minimal reproducers.

`workflow-reference-failure.json` records the remaining history by exact fixture digest. C# throws `ArgumentOutOfRangeException` for an unresolved `Inherit` increment on a tagged hotfix after child traversal stops. Go rejects that calculation too. The harness requires exit 1, empty stdout, the exact fixture identity and unchanged input; unexpected success, other errors/exits, timeout, output or mutation cannot count as this rejection. Diagnostic wording differs and remains a CLI compatibility gap. This one case is reported separately from 999 successful version comparisons, never as a generated version match.

## Verification

All final gates passed on macOS arm64 and Ubuntu 26.04.1 arm64:

| Gate | macOS | Ubuntu |
| --- | --- | --- |
| Original curated / policies / resets | 63 / 250 / 192 | 63 / 250 / 192 |
| New workflow / edge checkpoints | 166 / 13 | 166 / 13 |
| Generated workflows: version JSON | 999 | 999 |
| Generated workflows: pinned rejection | 1 | 1 |
| Earlier generated calculation / policy / reset suites | 1,000 / 1,000 / 1,000 | Earlier generated suites not rerun in this batch |

The regular suite passed **1,048 tests/subtests across nine packages**, zero failures or skips. Vet (including all acceptance tags), module verification, formatting and diff checks passed. Candidate dependencies contain no CGO packages. The versioned [evidence manifest](WORKFLOW-EVIDENCE.json) records exact test names, run directories, report hashes, source identity and binary hashes.

The combined macOS command ended with one C# effective-configuration preflight timeout in `TestWorkflowEdgeParity`. It did not reach a version comparison. The failed run is retained; a focused fresh rerun passed all 13 cases with the same 30-second process timeout. Earlier gates were already green and were not repeated. This is a passing final gate set after a focused retry, not a claim that the initial combined command exited successfully.

Linux evidence is copied under `.local/workflow-ubuntu-nativefs-evidence/`; the temporary container was removed after verifying all six reports (1,683 JSON matches plus one pinned rejection). The candidate hashes below apply to every recorded gate. Subsequent checkpoint changes are documentation, package comments and the portable dashboard snapshot only.

All process comparisons run C# and Go separately against independent repository copies. Ubuntu uses the unchanged published C# assemblies, records its own runtime/host hashes, and compares fresh C# output with portable macOS reference outputs. The container is Ubuntu 26.04.1 arm64, image `sha256:4e39440ade4565b61cd7c01e4572b117cce23cc98d3db437beac6f133c5598eb`, with runtime 10.0.14, and runs with `--network none`.

Reproduction (set absolute `GO_HOST` and `DOTNET_HOST` to the pinned local installations):

```sh
CGO_ENABLED=0 "$GO_HOST" build -o .local/gitversion-go ./cmd/gitversion-go
CGO_ENABLED=0 "$GO_HOST" test -count=1 ./...
CGO_ENABLED=0 PARITY_DOTNET="$DOTNET_HOST" "$GO_HOST" test \
  -tags integration,policies -run '^Test(PinnedGoCalculation|ExpandedPolicy|ResetPolicy|Workflow|WorkflowEdge)Parity$' -count=1 -timeout 20m ./cmd/parity
CGO_ENABLED=0 PARITY_DOTNET="$DOTNET_HOST" "$GO_HOST" test \
  -tags expansion -run '^TestGeneratedWorkflowParity$' -count=1 -timeout 20m ./cmd/parity
```

For Ubuntu, cross-build the candidate and `go test -tags platform -c ./cmd/parity` with `GOOS=linux GOARCH=arm64 CGO_ENABLED=0`. The platform test executable runs from `cmd/parity`, with absolute `PARITY_DOTNET` and `PARITY_CANDIDATE` paths. Run `^TestPlatform(Curated|Policy|Reset|Workflow|WorkflowEdge|GeneratedWorkflow)Parity$`. The generated platform gate also requires `.local/workflow-generated-reference.json`, containing the 999 independently captured macOS C# outputs; the rejection is the separately pinned contract above. Evidence stays in `.local/platform-evidence/` and never enters Git.

## Recorded implementation and binaries

Implementation checkpoint: `6ce9921c6`; previous A–E checkpoint: `6f0ec4dda`. Tests record binary identity before and after every acceptance run. `.local/workflow-build-provenance.json` also records production-file hashes, baseline lock identity and the implementation checkpoint.

| Candidate | SHA-256 |
| --- | --- |
| macOS arm64 | `ade39741b8540f6843134a148cb81b158ab3b113961a04ecb674f64dbce1f4ca` |
| Ubuntu arm64 | `c84ae42845c4fa1179afe676ee769a7b362ce617f87cbaa0d4eba3183f884826` |

The final Linux run uses the container's native filesystem. Earlier shared-mount runs passed their completed curated groups but were interrupted before the full matrix finished; they are not counted as final evidence. Source, images, binaries, process outputs and checkpoint commits remain local. No Windows validation, CodeRabbit review, push or publication was performed.
