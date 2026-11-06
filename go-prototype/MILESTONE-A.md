# Milestone A local implementation

Implemented on November 6, 2026 in the isolated worktree `/Users/artur/.codex/worktrees/go-prototype-milestone-a/gitversion`, branch `codex/go-prototype-milestone-a`. Changes remain local and uncommitted. All implementation files are confined to `go-prototype/`; the original checkout and C# implementation are unchanged.

The prototype provides a pinned C# oracle, 25 deterministic go-git fixture scenarios with 41 checkpoints, a portable reference containing all 33 JSON output fields per checkpoint, and a process-based parity harness. There is no Go version calculator in this milestone.

## Reference identity

| Input | Pin |
| --- | --- |
| Source | `14d222235b7dabb7033473528a76fd18478a9217`, established `src/` CLI |
| SDK | `11.0.100-rc.3.26522.148` |
| Build | Release, net10.0, extracted source archive |
| Runtime | Explicit `--fx-version 10.0.14`; runtime file hashes verified |
| Backend | Managed; v7 parser and configuration |
| Go dependency | `github.com/go-git/go-git/v5 v5.19.3` |
| Local Go toolchain | Go 1.27.3, verified official archive SHA256 |
| Go execution | `CGO_ENABLED=0`; compiled dependency closure has no CgoFiles |

The historical C# build still packages its existing libgit2 fallback. Runs explicitly select the managed backend; the Go module has no libgit2 dependency. The reference build was not modified to remove historical packages.

## Requirement evidence

| Requirement | Evidence |
| --- | --- |
| Isolated local work | `codex/go-prototype-milestone-a`; only new `go-prototype/` files; no commits, pushes or PRs |
| Pin C# source, build and runtime | `baseline.json`, `scripts/build-baseline.sh`, private `.local/baseline.lock.json`; `TestBaselineSpecRejectsUnpinnedIdentity`, `TestRuntimeDirectorySelectsExactVersion` |
| Reject reference drift | `TestVerifyBaselineRejectsTamperingBeforeExecution`, `TestVerifyBaselineRejectsSpecAndHostDrift`, `TestFileSetDetectsAddedRemovedAndChangedFiles` |
| Reproducible real repositories with go-git | `TestCorpusDeterministicAndClean`, `TestGoldenCorpusMatchesRecipes`; fixed fixture identities in `testdata/reference.json` |
| Correct graph and tag construction | `TestAnnotatedTagsAndTrueMerge`, `TestCheckpointHasNoFutureRefs` |
| Fail on invalid fixture inputs | `TestGenerateRejectsInvalidRecipesAndExistingOutput`, `TestLoadRejectsDuplicateAndUnsafeIDs`, `TestCopyRefusesSymlinks` |
| Compare every field and type | `TestCompareStrictJSON`, `TestCompareReportsDeterministicFieldOrder`; typed comparisons include versions, sources, counts, null/missing/empty and large integers |
| Reject malformed output | `TestObjectRejectsInvalidJSON`; includes duplicate keys, invalid UTF-8, unpaired surrogate escapes, trailing data and non-object roots |
| Isolate executions and preserve inputs | `TestRunIsolatedProcess`, `TestStateDetectsWorktreeRefsAndIndexMutations`, `TestResolveRequiresExplicitArgumentAndAbsoluteExecutable` |
| Fail on process errors, timeout, mutation and excessive output | `TestRunRejectsFailedMalformedTimedOutAndMutatingCandidates`, `TestRunRejectsNonpositiveTimeout`; helper subprocesses exercise real execution paths |
| Capture and replay the C# reference | `TestPinnedReferenceCaptureReplayAndSeededMismatch`: 41/41 checkpoint replay, all fields compared against portable reference |
| Prove wrong outputs fail | Same integration test: 41/41 intentionally wrong candidate cases fail with field differences; tampered reference gives exactly 1 failure and 40 passes |
| Preserve reviewed reference data and export provenance | `TestWriteNewJSONPreservesReviewedData`, `TestMatchCasesRejectsMissingReorderedAndChangedIdentities`; integration test exports valid capture and rejects tampered export |

## Validation

The unit suite passed with 163 passing test/subtest events, no failures or skips. The C# integration acceptance test passed, including deliberate negative runs; those expected negative results are asserted by the test and are not test failures.

Commands used from `go-prototype/`:

```bash
CGO_ENABLED=0 go test -count=1 ./...
CGO_ENABLED=0 go vet ./...
CGO_ENABLED=0 go build ./cmd/parity
go mod verify

CGO_ENABLED=0 PARITY_DOTNET="/absolute/path/to/dotnet" go test \
  -tags integration -count=1 -json ./... > .local/validation-tests.jsonl
```

Build, vet and module verification passed. The final combined run passed all 165 tests/subtests with no failures or skips, recorded in `.local/validation-tests.jsonl`. Final integration evidence is retained under `.local/integration-evidence/run-1016419432/`; an intact successful capture is also retained at `.local/capture-b/`. The initial failed capture and debugging runs remain separate from the successful reference.

Local test-quality review checked assertions against observable outcomes and likely mutations. No production mutation campaign was run. Two demonstrated defects were corrected through failing tests: an early version assertion hid field-level mismatch evidence, and an embedded buffer exposed a fast copy method that bypassed the output cap. Negative tests now exercise both paths.

CodeRabbit was initially attempted with explicit scope `--agent --uncommitted --include-untracked --dir go-prototype`. The service rejected authentication and returned no findings. The user subsequently requested skipping CodeRabbit; external review is now intentionally skipped, not reported as passing. No further authentication or review attempts were made.

## Boundaries and next work

Tests ran on macOS arm64. Linux and Windows execution, broader policy coverage, and a Go calculator remain later milestones. Index stat-cache metadata is excluded from repository identity; logical index contents and repository bytes are checked. Candidate programs are trusted local executables, not sandboxed processes.

Milestone B has since implemented the go-git repository adapter and compared its observations with the pinned managed C# backend; see [MILESTONE-B.md](MILESTONE-B.md). The original installed-Git prototype approach is superseded by the go-git requirement. Version calculation remains Milestone C.

See [README.md](README.md) for reproducible commands, manifest semantics, fixture format, and connecting a future Go calculator.
