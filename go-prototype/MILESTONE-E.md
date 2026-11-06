# Milestone E — first configuration expansion batch

This batch extends the independent go-git calculator beyond the original frozen GitHub Flow defaults. Everything remains local and uncommitted. Windows execution is explicitly deferred; the original D three-platform gate remains open. CodeRabbit is skipped at the user's request.

## Support contract

The pinned reference remains the established `src/` C# implementation at `14d222235b7dabb7033473528a76fd18478a9217`, with v7 configuration and the managed repository backend. No C# calculation code, reference JSON, libgit2 or Git subprocess is needed by the Go executable at runtime.

The following overrides are supported for the existing `main`/`master` and feature branch families:

| Group | Accepted settings |
| --- | --- |
| Deployment | `ManualDeployment`, `ContinuousDelivery`, `ContinuousDeployment` on both branch kinds |
| Increments | Main and feature `None`, `Patch`, `Minor`, `Major`; feature `Inherit` |
| Labels | Literal string labels, including empty/numeric strings; feature `{BranchName}` substitution |
| Output weights | `output.tag-pre-release-weight` and `output.branches.<kind>.pre-release-weight`, including explicit zero |
| Tagged commits | `calculation.branches.<kind>.prevent-increment.when-current-commit-tagged` |
| Commit messages | `Enabled`, `Disabled`, `MergeMessageOnly` under each calculation branch |

See [README.md](README.md) for raw YAML. Weights belong to the v7 **output** section; deployment, label and increment settings belong to **calculation**. Defaults retain the old behavior. Explicit feature increments stop source-branch configuration inheritance. Manual deployment retains build metadata, continuous delivery advances the prerelease count, and continuous deployment removes the prerelease from the displayed version while preserving semantic-source identity.

Unknown fields, other branch families, invalid enums, incorrectly typed booleans/weights, negative/overflowing weights, unsupported placeholders and main `{BranchName}` are rejected. The main regex has no BranchName capture in the pinned C# implementation. Go provides diagnostics for this rejection; C# exits silently. Error message text is not a compatibility promise.

## Independent verification

The new deterministic corpus has **50 recipes and 250 checkpoints**, covering before/at/after tags, ordinary commits and true merges. C# outputs were captured first, validated and frozen separately as `testdata/policy-reference.json`. Existing A/C/D goldens were not regenerated. Every successful comparison checks all **33 JSON fields**, their types, semantic/counting sources, weights and null values, plus unchanged repository state. Raw YAML is read independently by each executable.

- Fresh C# capture and built Go candidate: **250/250** each; `.local/policy-evidence/run-3251068010/`.
- New seeded configuration/history combinations: **1,000/1,000**, seed `20261106`; `.local/expanded-generated-evidence/run-2508203275/`.
- Invalid mode, invalid increment and unavailable main label capture: **3/3** rejection cases; `.local/milestone-e-failures.log` records the retained run. Both exit 1, emit no version JSON and leave input unchanged. Go diagnostics are required.
- Original generated corpus rerun: **1,000/1,000**; `.local/expansion-evidence/run-1255995569/`.
- Linux arm64: **250/250** new policies and **63/63** original curated cases; `.local/platform-evidence/run-3300271910/` and `run-3368574658/`.
- Combined integration suite: **680 tests/subtests across nine packages**, zero failures/skips; `.local/milestone-e-tests.jsonl`. Includes fresh A harness/tamper checks, B observation parity, C 63-case parity, D reduced regressions and E frozen reference. Separately run policy/generated/platform/public cases are not added to this test count.
- Public history replay: **6/6** with repeated relocated/environment checks; `.local/public-evidence/run-1170523197/`. An initial invocation lacked the required local-source mapping and stopped at preflight; the corrected invocation passed.
- Build, all-tag vet, module verification, gofmt and whitespace checks passed. CGO was disabled and the CLI dependency closure contains no CgoFiles.

A preliminary C# capture rejected five checkpoints using the unsupported main capture template (`.local/policy-evidence/run-2331457767/`). Those inputs were moved outside the successful corpus and the failure behavior is tested explicitly. The first error-path test also incorrectly required C# stderr for that rejection; the retained failed run documents the silent baseline behavior. Neither failed run is counted as a pass. The 64-case generated pilot is separate from the final 1,000-case result.

| Requirement | Evidence |
| --- | --- |
| All modes on both branches, counts and metadata | `TestDeploymentModesControlNumberAndMetadata`, `TestExpandedPolicyParity` |
| Effective settings, inheritance, explicit empty/zero/false | `TestExpandedPolicyEffectiveSettings`, `TestExpandedPolicyParity` |
| Tagged-head and message-policy interactions | `TestTaggedFeaturePolicyAndDisabledMessages`, `TestExpandedPolicyParity` |
| Weight boundaries, overflow and metadata | `TestOutputExplicitWeightsAndDeploymentMetadata`, `TestExpandedPolicyRejectsInvalidValues` |
| Strict invalid-input rejection and immutable inputs | `TestInvalidPolicyFailureParity`, `TestExpandedPolicyRejectsInvalidValues` |
| Frozen independent 33-field reference | `TestExpandedPolicyFrozenReference` |
| Deterministic raw configuration corpus | `TestPolicyCorpusAndGeneratedOverrides` |
| New generated policy/history parity | `TestGeneratedPolicyParity` |
| Original generated, public and curated regression scopes | `TestGeneratedCalculationParity`, `TestPublicHistoryReplayAndRepeatability`, `TestPinnedGoCalculationParity` |
| Linux execution | `TestPlatformPolicyParity`, `TestPlatformCuratedParity` |

## Reproduce locally

From this module, use the same absolute Go and .NET hosts recorded by the baseline lock. Build the candidate before process acceptance; do not replace it during a run.

```bash
CGO_ENABLED=0 "$GO_HOST" build -o .local/gitversion-go ./cmd/gitversion-go
CGO_ENABLED=0 PARITY_DOTNET="$DOTNET_HOST" "$GO_HOST" test -tags policies -count=1 -timeout 20m ./cmd/parity
CGO_ENABLED=0 PARITY_DOTNET="$DOTNET_HOST" "$GO_HOST" test -tags expansion -run '^TestGenerated(Calculation|Policy)Parity$' -count=1 -timeout 30m ./cmd/parity
CGO_ENABLED=0 PARITY_DOTNET="$DOTNET_HOST" "$GO_HOST" test -tags integration -count=1 -timeout 20m ./...
CGO_ENABLED=0 "$GO_HOST" vet -tags integration,policies,expansion,platform ./...
```

Linux uses a CGO-disabled Linux arm64 candidate/test runner, the identical published C# assemblies, and independently recorded Linux host/runtime hashes. It compares against the portable golden as well as fresh C# output. The local container is pinned to `mcr.microsoft.com/dotnet/runtime@sha256:4f1f30cf0bb4ab0c8b2daa84613051683456f8df4a9bf7c812e20d74962992ac` and runs with networking disabled. Windows is not executed in this milestone.

### Ubuntu 26.04 rerun

At the user’s request, the same Linux candidate and test runner were also verified in Ubuntu **26.04.1 LTS** (arm64). `scripts/Dockerfile.ubuntu26` pins the Ubuntu base and copies the pinned .NET 10.0.14 runtime. The local built image ID is `sha256:4e39440ade4565b61cd7c01e4572b117cce23cc98d3db437beac6f133c5598eb`; OS release, installed package versions, image metadata and Dockerfile digest are retained in `.local/ubuntu26-environment/`. Package installation uses networking during image construction; test execution disables networking. **Passed: 63/63 original checkpoints and 250/250 new policy checkpoints.** Evidence: `.local/platform-evidence/run-1778595297/` and `run-1260552609/`. Both compare fresh C# and Go output field by field.

```bash
docker build --platform linux/arm64 -f scripts/Dockerfile.ubuntu26 -t gitversion-go-parity:ubuntu26 scripts
# Use the previously CGO-disabled cross-built Linux candidate and platform test binary.
docker run --rm --network none \
  --mount type=bind,source="$PWD",target=/work --workdir /work/cmd/parity \
  -e PARITY_DOTNET=/usr/share/dotnet/dotnet \
  -e PARITY_CANDIDATE=/work/.local/platform-bin/linux-arm64/gitversion-go \
  --entrypoint /work/.local/platform-bin/linux-arm64/parity.test \
  gitversion-go-parity:ubuntu26 \
  -test.run '^TestPlatform(Curated|Policy)Parity$' -test.v -test.timeout 20m
```

The recorded run uses the immutable built image ID rather than its local tag. Rebuilding later may resolve newer Ubuntu packages; record the new image/package provenance instead of assuming byte-identical images.

macOS candidate SHA256: `c1b33493af371824027c7263deb24001704606987ff2e7f3f104ef287ea0a5a6`. Baseline lock SHA256 remains `cfbc6c9fb49026cb16087eb1c53c8b8ec69b1da448aa1fdaf1f7b44c8791d79a`. Full stdout/stderr, command arguments, environment, hashes, repository copies and comparison reports remain under ignored `.local/` evidence directories.

## Remaining compatibility work

This completes the declared first E batch. GitFlow/release/hotfix branches, Mainline, arbitrary regex/configuration, additional label placeholders, CI normalization/fetch, shallow/bare/dirty/detached inputs, caching, build-server output, MSBuild and assembly updates remain unsupported. The conservative Int32 version/prerelease limits still apply even when a smaller weight would permit a larger value in C#. Success on this corpus is not a percentage of all GitVersion behavior or a general drop-in replacement claim.

Dashboard benchmark numbers remain the historical D snapshot. The E public replay can collect timings under concurrent validation load; those samples are correctness evidence, not a new comparative performance claim.

## Subsequent work

The user has confirmed full replacement including CLI and integrations, with Windows validation last. See [COMPATIBILITY.md](COMPATIBILITY.md) for the remaining inventory and [MILESTONE-E-RESETS.md](MILESTONE-E-RESETS.md) for the subsequent reset-message batch. Results in this report remain tied to the first-batch candidate; its binary is retained separately.
