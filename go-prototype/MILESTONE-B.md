# Milestone B — Go repository adapter

Implemented locally on November 6, 2026, in the existing isolated `codex/go-prototype-milestone-a` worktree. No commits, pushes, upstream edits, or external reviews. CodeRabbit is skipped at the user's request.

## Result

All **47 repository snapshots match the pinned C# managed backend**: the 41 Milestone A checkpoints and six adversarial graphs. This includes **2,233 ordered-history and merge-base query results**, plus the complete observed references, tags, commit metadata, first-parent changed paths, branch identity and clean state.

At the Milestone B boundary, the Go command was `gitversion-go observe --repository PATH`; version calculation was not yet implemented. Milestone C subsequently added independent GitHub Flow calculation; see [MILESTONE-C.md](MILESTONE-C.md). The repository-observation evidence below remains a distinct gate.

## Implementation

- `internal/gitrepo`: go-git object access; C#-compatible walk sorting, reachability exclusion, first-parent queries and merge-base selection. No Git subprocesses or libgit2 dependencies.
- `cmd/gitversion-go`: read-only observation CLI.
- `oracle/RepositoryObserver`: a small C# probe which loads the unchanged pinned managed assemblies. Reflection is needed only to construct the internal adapter. A rejecting proxy prevents mutating or network Git calls.
- `scripts/build-observer.sh`: builds the probe separately under ignored `.local/` and builds the pure Go observer. It does not alter the frozen reference executable.

The pinned baseline remains `14d222235b7dabb7033473528a76fd18478a9217`, Release/net10.0, runtime 10.0.14. Original baseline provenance and portable expected-version JSON are unchanged.

## Requirement evidence

| Requirement | Evidence |
| --- | --- |
| Real C# observations on the same histories | `TestPinnedRepositoryObservations`: 47/47 snapshots, separate repository copies/processes, baseline checksums, no mutation |
| Walk ordering, reverse, first-parent and exclusions | `TestWalkOrderingAndExclusion`; acceptance exercises all 8 sort combinations and both first-parent settings |
| Equal timestamps and clock skew | `TestEqualTimestampsAndSkewedTopology`; acceptance includes equal-date diamond, long skewed chain and tied criss-cross graph |
| Merge bases, unrelated histories and competing best bases | `TestMergeBase`; acceptance compares every selected tip pair in both directions |
| Tag peeling, metadata and changed paths | `TestTagsAndDiffPaths`, including nested annotations and independent parent/message/time assertions |
| Reference iteration compatibility | `TestPackedReferenceOrdering`, `TestUnicodeReferenceOrder`; packed and Unicode fixtures also run against C# |
| Explicit unsupported-state and missing-object failures | `TestOpenRejectsUnsupportedStates`, `TestUnsupportedReferencesFailClosed`, `TestNonCommitTagFailsClosed`, `TestMissingParentFailsClosed` |
| Observation comparator catches wrong answers | `TestObservationComparatorDetectsSeededChanges`: wrong order, wrong base, missing parent and missing reference |
| CLI emits observations and rejects calculation | `TestObserveCommand`, `TestUnsupportedCommandProducesNoJSON` |

## Evidence and reproduction

The final combined run passed **251 tests/subtests with zero failures or skips** across all five Go packages. This includes Milestone A's capture/replay, wrong-candidate rejection and reference-tamper acceptance test. `CGO_ENABLED=0 go build ./...`, `go vet ./...`, and `go mod verify` passed; the compiled Go CLI dependency closure has no CgoFiles. The C# observer build completed with zero warnings/errors.

Test events: `.local/milestone-b-tests.jsonl`. Fresh Milestone A acceptance evidence: `.local/integration-evidence/run-3329625709/`. The old Milestone A result remains as historical evidence.

The successful observation run is `.local/observation-evidence/run-349732172/report.json` (`complete: true`, 47 passed, zero failed). Each case retains both JSON observations, process metadata, stderr, strict differences and independent repository copies. The report records baseline-lock, recipe, observer-source/binary and Go-executable hashes.

Reproduction commands are in `README.md`. Use `CGO_ENABLED=0`, the pinned local dotnet host, and `-tags integration` to include both acceptance suites. Missing acceptance prerequisites fail rather than skip.

## Scope and review

Verified on macOS arm64 with Go 1.27.3; cross-platform and large-history performance validation remain pending. The adapter accepts clean, complete local repositories with attached HEAD and ordinary `.git` directories, local branches and commit-targeting tags. Other states fail explicitly. It is a read-only, non-concurrent snapshot interface; callers must not mutate files beneath it.

An assertion and test-gap review identified and corrected UTF-16 ordinal reference ordering, which differs from Go byte ordering for some Unicode names. Unit checks and a new live C# comparison protect it. No empirically executed mutation score is claimed. The comparison corpus proves the declared observations; it does not establish universal Git behavior or version-output compatibility. Unusual tree transitions, arbitrary repository encodings, normalization, remote preparation, configuration interpretation and version policies remain future work.
