# Milestone E — reset-message expansion

The user confirmed a full replacement target, including CLI and integrations, with macOS and Ubuntu prioritized and Windows last. This batch removes one explicit calculation limitation: default `=semver:` reset messages. It does not complete the full replacement; [COMPATIBILITY.md](COMPATIBILITY.md) and [CLI-COMPATIBILITY.md](CLI-COMPATIBILITY.md) track remaining groups.

## Behavior

The calculator scans eligible messages in the pinned C# topological/time order. It accumulates the maximum recognized bump from newer commits, includes the first recognized reset, then ignores older messages. A reset can lower the configured branch increment; without a recognized reset, that configured floor still applies. A reset marker with no recognized bump does not terminate the scan. No-bump/skip takes precedence over major/minor/patch directives in the same commit. Disabled and merge-only message policies apply before accumulation, and commits covered by matching tags are excluded first.

The implementation remains Go/go-git, with no subprocess/runtime reference dependency. All prior accepted raw configuration and default behavior remains supported. Custom reset regex is still outside the declared configuration subset.

## Reference and tests

- New independent reference: `testdata/reset-reference.json`; the pinned C# executable captured **72 deterministic recipes / 192 checkpoints** before changing calculation behavior.
- Histories cover main/feature, Enabled/Disabled/MergeMessageOnly, before/at/after resets, older major/newer minor messages, reset None/Patch/Minor/Major, unrecognized markers, mixed markers, case/Unicode whitespace, merge messages, tied dates and annotated-tag exclusion.
- Direct assertions failed before implementation because Go explicitly rejected reset messages; `.local/reset-red-test.log` retains that evidence.
- `TestResetPolicyFrozenReference` passes every captured field/type and checks immutable fixture identities and unchanged repository state. Original policy/original C#/reduced-regression goldens remain separate.
- `GeneratedResets` layers seeded reset messages onto the unchanged generated policy graphs. The 1,000-case run includes 947 histories with reset markers, 53 controls and 3,080 reset messages. Fixture tests verify determinism and that configuration, graph structure and operations are preserved.
- Fresh macOS process parity: **192/192**, `.local/reset-evidence/run-2441139663/`.
- Generated reset/configuration histories: **1,000/1,000**, `.local/reset-generated-evidence/run-1871208970/`, seed `20261106`.
- Combined integration suite: **881 tests/subtests across nine packages**, zero failures/skips, `.local/reset-combined.jsonl`. Separate generated/platform cases are not added to that total.
- Current candidate original Ubuntu cases: **63/63**, `.local/platform-evidence/run-3176565713/`. Corrected Ubuntu policy and reset suites passed **250/250** and **192/192**, `.local/platform-evidence/run-1267840145/` and `run-1093101991/`. Together with the original 63, this is **505 curated Linux checkpoints** on Ubuntu 26.04.1 LTS arm64.
- Build, all-tag vet, module verification and formatting pass. No CgoFiles in the CLI dependency closure.

| Requirement | Evidence |
| --- | --- |
| Resets lower the configured floor and ignore older bumps | `TestResetOverridesOlderMessagesAndConfiguredFloor` |
| Newer messages, tags, merges, policies and all output fields | `TestResetPolicyFrozenReference`, `TestResetPolicyParity` |
| Deterministic independent reset histories | `TestResetFixturesDeterministic` |
| Generated reset/configuration/history interactions | `TestGeneratedResetParity` |
| Ubuntu 26.04 native Linux execution | `TestPlatformResetParity` |
| Previous policy and calculation scopes | `TestExpandedPolicyFrozenReference`, `TestPlatformPolicyParity`, `TestPlatformCuratedParity`, `TestPinnedGoCalculationParity` |

## Execution

```bash
CGO_ENABLED=0 "$GO_HOST" build -o .local/gitversion-go ./cmd/gitversion-go
CGO_ENABLED=0 PARITY_DOTNET="$DOTNET_HOST" "$GO_HOST" test -tags policies -run '^TestResetPolicyParity$' -count=1 -timeout 15m ./cmd/parity
CGO_ENABLED=0 PARITY_DOTNET="$DOTNET_HOST" "$GO_HOST" test -tags expansion -run '^TestGeneratedResetParity$' -count=1 -timeout 30m ./cmd/parity
CGO_ENABLED=0 PARITY_DOTNET="$DOTNET_HOST" "$GO_HOST" test -tags integration -count=1 -timeout 20m ./...
```

The Ubuntu container and environment provenance are documented in [MILESTONE-E.md](MILESTONE-E.md). Cross-build the current Linux candidate and platform test executable, then include `TestPlatformResetParity` in its test filter. Windows is left unexecuted. This run retains its own candidate hashes; prior E batch evidence belongs to its preserved `gitversion-go-milestone-e-first-batch` binary.

The strengthened Linux inventory gate caught a harness map-reuse defect before comparisons: decoding a new golden into an existing map retained the earlier corpus entries. The runner now loads exactly one selected golden into an empty structure, checks inventory and requires every policy/reset checkpoint to exist. The failed run remains in `.local/reset-ubuntu26.log`; corrected execution is recorded separately.

Current candidate SHA256: macOS `82713982c01506c2d1c2d1eee074e31ea7f6b9569eea94ce4fe7959bb10afff1`; Linux `11b0a803fa8902ac3a4feb99a491f462a8f31d00d42bcad1c4c3f308366073ab`. Reset golden SHA256: `00aff6edfafc8413423401268c0e202427e87653550b2e051a2655b0412cfb1c`.

This reset batch is complete on the declared macOS/Ubuntu subset. The broader full-replacement target remains open; Windows remains pending. No commits, pushes, releases or VM operations were performed.
