# GitVersion Go prototype

[Portable progress dashboard](PROGRESS.html) · [Workflow parity report](WORKFLOWS.md) · [Evidence manifest](WORKFLOW-EVIDENCE.json)

Milestones A–C provide a pinned C# reference, deterministic fixtures, a strict parity harness, a go-git repository adapter, and an **independent Go calculator for a frozen GitHubFlow/v1 subset**. The original version corpus has 25 scenarios, 41 checkpoints, and 33 output fields per checkpoint. Calculation acceptance adds separate histories without changing that original reference.

Milestone D adds 1,000 seeded generated comparisons, six pinned public-history replays, 35 reduced regressions, Linux execution and local benchmarks. **Windows execution remains unverified**, so the original three-platform exit gate remains open. See [MILESTONE-D.md](MILESTONE-D.md) for measured scope and evidence. Milestone E expands main/feature deployment modes, increments, labels, weights and tag/message controls. See [MILESTONE-E.md](MILESTONE-E.md) for those historical batches. [WORKFLOWS.md](WORKFLOWS.md) describes the subsequent GitFlow/Mainline expansion and its historical evidence. [CONFIGURATION-STRATEGIES.md](CONFIGURATION-STRATEGIES.md) records the later strategy/control expansion, and [CSHARP-TEST-REUSE.md](CSHARP-TEST-REUSE.md) tracks reuse of the original tests.

The full-replacement target, including CLI and integrations, is tracked in [COMPATIBILITY.md](COMPATIBILITY.md). Windows validation is last. The completed E reset-message batch is documented in [MILESTONE-E-RESETS.md](MILESTONE-E-RESETS.md).

The module uses **go-git v5.19.3** and requires Go 1.26 or newer. Fixture construction, checkout, tags, index inspection, and repository-state checks use go-git. No Git subprocesses, cgo, or libgit2 are used by the Go harness. Native Git is used only by the shell script to extract the pinned C# source from this parent repository.

## Reference identity

`baseline.json` pins the established `src/` C# implementation at `14d222235b7dabb7033473528a76fd18478a9217`, its SDK, Release/net10.0 build, runtime 10.0.14, v7 configuration/parser, managed Git backend, and invocation. The prototype intentionally does not use the experimental `new-cli/` tree or published v6 behavior.

The C# oracle is an unmodified historical build. That source still references and publishes its legacy libgit2 fallback; the harness explicitly selects its **managed** backend. Those historical C# packages are confined to the ignored oracle build and are not Go dependencies. Eliminating the historical fallback from C# packaging is outside this prototype.

`scripts/build-baseline.sh` extracts exactly the pinned commit with `git archive`, builds the extracted source, captures help/schema/version, and writes `.local/baseline.lock.json`. The lock includes the source-archive SHA256, baseline-spec SHA256, every published file's SHA256, the dotnet-host SHA256, and every file in the explicitly selected .NET runtime. It also records SDK, toolchain, platform and runtime inventory. Runs verify the spec, executable directory, host and pinned runtime against this lock before executing. This detects drift; it is local build provenance, not a signed supply-chain attestation.

The builder refuses to overwrite an existing baseline, source directory or lock. Preserve previous evidence when making an intentional baseline migration. Never replace the pinned revision with a moving branch or global tool installation.

## Build and validate locally

Run from `go-prototype/`. Set absolute tool paths if the defaults do not resolve the pinned SDK or Go toolchain.

```bash
export DOTNET_HOST="/absolute/path/to/dotnet"
export GO_HOST="/absolute/path/to/go"
bash scripts/build-baseline.sh

CGO_ENABLED=0 "$GO_HOST" test -count=1 ./...
CGO_ENABLED=0 "$GO_HOST" vet ./...
CGO_ENABLED=0 "$GO_HOST" build ./cmd/parity

./parity capture --dotnet "$DOTNET_HOST" --out .local/capture
./parity verify --dotnet "$DOTNET_HOST" --expected .local/capture --out .local/replay

CGO_ENABLED=0 PARITY_DOTNET="$DOTNET_HOST" "$GO_HOST" test \
  -tags integration -count=1 -run '^TestPinnedReferenceCaptureReplayAndSeededMismatch$' \
  -v ./cmd/parity
```

All output directories must be new. Captures and replay reports remain under ignored `.local/`. Integration acceptance runs retain evidence there and fail if prerequisites are missing. No network operations are performed by the fixture/calculation runs; initial toolchain, module and NuGet restore need downloads. Candidate executables are trusted local programs, not sandboxed code.

The integration test captures C#, replays C# on independently rebuilt fixtures, compares every captured field against the checked-in portable reference, runs an intentionally wrong candidate, and checks that reference-output tampering is rejected. This establishes that the harness works; **C# replay is not Go calculation parity**.

## Fixtures and portable reference

`testdata/recipes.json` describes sequential operations and checkpoints. Commits use fixed identities, UTC dates, LF file bytes, sorted trees and explicit parents. Annotated tag identities/dates are fixed. Every checkpoint copies the repository at that moment, preventing future refs or tags from leaking into earlier cases. Configuration is committed as `GitVersion.yml`, keeping the working tree clean.

```bash
./parity fixtures --out .local/fixtures
./parity freeze --expected .local/capture --out .local/new-reference.json
```

`freeze` accepts only a complete successful capture with matching provenance, checkpoint identities and output checksums, and refuses an existing destination. It exports machine-independent fixture identities and typed JSON to a new portable reference. Review changes before deliberately updating `testdata/reference.json`; a mismatch never regenerates expected output automatically. `TestGoldenCorpusMatchesRecipes` pins the fixture identities independently of repeat generation. Several stable tagged checkpoints also have explicit version assertions adapted from the existing GitHub Flow integration example.

The portable reference records the baseline spec, recipe and original capture-lock digests. The complete machine-specific lock remains beside the capture. A fresh build on another platform can capture and compare with the portable reference without claiming its binaries have the original machine's checksum. The initial checked-in reference was captured on macOS arm64. The curated corpus now also runs on Linux arm64; Windows execution remains pending.

## Execution and comparison contract

- Each invocation receives its own repository copy and private HOME/temp/config directories. The environment is allowlisted, CI selectors and credentials are not inherited, and baseline selectors/telemetry opt-out are explicit. The exact argv and environment are recorded.
- Distinct raw configurations receive validation by the pinned executable and an effective v7 YAML capture. The configuration/strategy corpus also has a separate deduplicated CLI-validation audit; new captures run this preflight before calculation. Fetch, calculation cache, normalization and telemetry are disabled for version runs.
- Repository state before/after includes refs, objects, file contents, executable bits, and logical index entries. Index stat-cache timestamps and inode data are deliberately excluded because filesystem copies change them. This checks observable repository contents, not filesystem metadata such as access times or empty directories.
- JSON comparison requires exactly the same keys, types and values, including null/empty/missing distinctions and integers above 2^53. Only key order and whitespace are ignored. Number spellings are also preserved: `1` differs from `1.0`. Duplicate keys, malformed JSON, invalid UTF-8, empty objects, non-object roots and trailing data fail.
- Every output field is compared, including semantic sources and commit-counting anchors/distances. Comparing only `FullSemVer` is insufficient.
- Nonzero exit, launch failure, timeout, repository mutation, malformed output, changed provenance or missing reference data fail closed. A capture with failed calculation cases has a report but no usable `corpus.json`; preflight failures stop immediately with their available command evidence. No unsupported or skipped cases count as passing. The workflow stress suite separately verifies one exact, digest-pinned C# rejection; it is reported as a rejection contract, not a version match (see [WORKFLOWS.md](WORKFLOWS.md)).
- Each case retains stdout, stderr, exit code, duration, argv/environment and repository-state identities. Verify reports list field differences. Output is capped at 4 MiB per stream; exceeding the cap fails the run. Per-process timeout defaults to 30 seconds. Child-process trees are not sandboxed or guaranteed to be terminated; use trusted candidates only.

## Milestone B: repository observation parity

`internal/gitrepo` reads repository objects with go-git and implements the pinned backend's history ordering and single-merge-base selection. `cmd/gitversion-go` exposes these observations as JSON:

```bash
CGO_ENABLED=0 "$GO_HOST" build -o .local/gitversion-go ./cmd/gitversion-go
.local/gitversion-go observe --repository /absolute/path/to/clean/repository
```

The current adapter accepts complete, clean repositories with a normal `.git` directory, attached local HEAD, local branches, and commit-targeting tags. It rejects bare/empty/shallow/dirty/detached repositories, linked-worktree inputs, remote or other reference namespaces, symbolic refs other than HEAD, and tags targeting non-commits. Missing reachable objects fail instead of truncating history. This is a deliberate prototype boundary, not full repository-state support. Instances are read-only and not safe for concurrent use or external repository mutation.

The observation contract includes raw references in loose-then-packed C# ordinal order, branch identity, one-level tag targets and recursively peeled commits, commit parents/messages/timestamps, first-parent changed paths, all eight sort-flag combinations with and without first-parent traversal, each branch as an exclusion root, and merge bases in both argument directions. Equal-time ordering uses explicit compatibility heap behavior rather than go-git's default log order.

The C# probe in `oracle/RepositoryObserver` invokes the **existing pinned published assemblies** through the public repository interface; it does not copy or rebuild their algorithms. Reflection constructs the internal managed adapter with a mutation-rejecting proxy. It has no package dependencies. The Go adapter runs in a separate process and does not consume C# observations at runtime.

```bash
# Reuse the absolute DOTNET_HOST and GO_HOST selected for Milestone A.
bash scripts/build-observer.sh
CGO_ENABLED=0 PARITY_DOTNET="$DOTNET_HOST" "$GO_HOST" test \
  -tags integration -count=1 -run '^TestPinnedRepositoryObservations$' ./internal/gitrepo

# Combined unit and acceptance checks, including Milestone A replay/tampering:
CGO_ENABLED=0 PARITY_DOTNET="$DOTNET_HOST" "$GO_HOST" test -tags integration -count=1 ./...
```

The acceptance test checks baseline integrity, fingerprints the observer binaries/source and Go executable, and compares independent process runs on all 41 original checkpoints plus six adversarial snapshots: equal dates, skewed dates, criss-cross merges with distinct and tied dates, packed/nested tags, and Unicode reference ordering. The current set compares **47 snapshots and 2,233 ordered-walk/merge-base query results**, plus exact metadata, references, tags and changed paths. Both processes receive isolated environments and repositories; their before/after state must agree. Evidence is retained under `.local/observation-evidence/run-*/`, with `report.json`, stdout, stderr, process records, differences and repository copies. A report is complete only when every expected case passes.

See [MILESTONE-B.md](MILESTONE-B.md) for the validated scope and requirement evidence. Arbitrary repository layouts, unusual tree transitions, scale/performance, and other platforms need further cases. This result establishes repository-input parity for the tested scope, **not Go version-output parity**.

## Milestone C: independent version calculation

Build and run the pure Go calculator from this module:

```bash
CGO_ENABLED=0 "$GO_HOST" build -o .local/gitversion-go ./cmd/gitversion-go
.local/gitversion-go calculate --repository /absolute/path/to/repository --output json
.local/gitversion-go calculate --repository /absolute/path/to/repository --output json --trace
```

`calculate` reads the repository and its raw YAML independently. It does not read the reference corpus, call C#, run Git subprocesses, fetch, normalize, cache, or write repository contents. `--trace` writes a separate JSON object to stderr with branch settings, candidates, semantic source, counting anchor and counted commits; stdout remains the 33-field version JSON.

The initial C subset is extended by Milestone E. Example supported raw v7 configuration:

```yaml
workflow: GitHubFlow/v1
calculation:
  next-version: 3.0.0
  branches:
    main:
      increment: Minor
    feature:
      mode: ContinuousDelivery
      increment: Inherit
      label: 'preview.{BranchName}'
      commit-message-incrementing: MergeMessageOnly
      prevent-increment:
        when-current-commit-tagged: true
output:
  tag-pre-release-weight: 60000
  branches:
    feature:
      pre-release-weight: 30000
```

Both `main` and `feature` support `mode` (`ManualDeployment`, `ContinuousDelivery`, `ContinuousDeployment`), `increment` (`None`, `Patch`, `Minor`, `Major`; feature additionally `Inherit`), literal string labels, `commit-message-incrementing` (`Enabled`, `Disabled`, `MergeMessageOnly`) and boolean `prevent-increment.when-current-commit-tagged`. Feature labels additionally support `{BranchName}`. Output weights accept nonnegative decimal integers within Int32; explicit zero is preserved. `next-version` accepts three-component stable and prerelease versions; the new reference matrix covers matching/mismatching numeric prerelease labels and strategy competition. Omitted settings retain the original GitHub Flow defaults.

The later [workflow expansion](WORKFLOWS.md) extends these controls to additional preset branches. The [strategy/control expansion](CONFIGURATION-STRATEGIES.md) adds all seven explicit strategy flags, empty/None/duplicate selections, main/release classification, release/merge tracking, and `prevent-increment.of-merged-branch` / `when-branch-merged`. Competing source configurations and Mainline inheritance transitions are preserved. Only declared fields and branches are accepted; unsupported settings fail explicitly. YAML aliases/anchors, duplicate keys, multiple documents and incorrectly typed settings are rejected. Configuration discovery uses the C# filename priority and case-insensitive matching among the four standard names at the repository root. Missing or empty configuration uses GitFlow defaults. Pass `--config PATH` to select an existing relative or absolute configuration file. YAML uses the checksum-pinned `gopkg.in/yaml.v3 v3.0.1` dependency. See [MILESTONE-E.md](MILESTONE-E.md) for validation commands and limitations.

The calculator accepts at most one `main` or `master` branch. Explicitly configured branches can calculate without a main ref; inherited branches still require a source. The workflow expansion adds the preset develop, release, hotfix, support and pull-request families described in [WORKFLOWS.md](WORKFLOWS.md), alongside the original feature-prefix pattern. Defaults remain main ContinuousDelivery and feature ManualDeployment; all three modes can now be selected explicitly. It supports strict version tags with optional `v`/`V` prefix, matching and unrelated prerelease labels, configured next versions, fallback, ordinary merges, and default `+semver:`/`=semver:` bump and reset directives. Semantic-source and commit-counting-source identities are kept separate. Supplementary Unicode characters follow C# UTF-16 sanitization, and bump-message whitespace follows .NET's Unicode rules.

In addition to the repository adapter's rejected states, calculation explicitly rejects octopus merges, ambiguous main branches and unsupported topology. Numeric components are bounded to Int32-compatible output, with room for prerelease weights; out-of-range calculations fail. This is not a general GitVersion configuration implementation.

Run the independent frozen comparison and the live acceptance gate:

```bash
CGO_ENABLED=0 "$GO_HOST" test -count=1 ./internal/calculate
CGO_ENABLED=0 PARITY_DOTNET="$DOTNET_HOST" "$GO_HOST" test \
  -tags integration -count=1 -run '^TestPinnedGoCalculationParity$' ./cmd/parity
```

The live gate captures fresh C# outputs for the original 41 checkpoints and 22 extra checkpoints, then runs the built Go executable through the existing strict process harness. It also checks the fresh original outputs against the immutable portable reference, runs a relocated repository with an empty `PATH`, verifies unchanged repository state and checks trace separation. Evidence is retained under `.local/calculation-evidence/run-*/`, including recipes, executable/lock hashes, both outputs, traces, process records and diffs. `provenance.json` becomes complete only after all checks pass. See [MILESTONE-C.md](MILESTONE-C.md).

## Running an explicit candidate comparison

Create a command specification using an absolute executable path and a standalone `{repo}` argument:

```json
{
  "executable": "/absolute/path/to/gitversion-go",
  "arguments": ["calculate", "--repository", "{repo}", "--output", "json"]
}
```

```bash
./parity verify --dotnet "$DOTNET_HOST" --expected .local/capture \
  --candidate .local/candidate.json --out .local/go-comparison
./parity compare --expected expected.json --actual actual.json
```

The harness passes arguments directly, without shell expansion. A candidate receives the same tracked raw configuration and isolated repository state. C# is needed for reference capture and verification, but is not a runtime dependency of `gitversion-go calculate`.

## Current boundaries

The corpus is a starting reference set, not a claim of complete GitVersion compatibility. GitFlow/Mainline and all three deployment modes now have separate bounded reference corpora. It does not yet cover arbitrary regex/formatting configuration, dirty or shallow repositories, detached HEAD, remote preparation, CI outputs, or MSBuild. Those belong to later milestones with additional reference cases. No upstream code, packaging, tests or workflows are changed by this module.

## Milestone D: generated histories and public replay

The opt-in `expansion` build tag runs expensive experiments separately from the regular suite. Every generated mismatch is a failure, not an unsupported/skip classification. Set `PARITY_GENERATED_COUNT` only for exploratory pilot runs; the D gate requires at least 1,000. All recipes, repository snapshots, independent process outputs and provenance are retained in `.local/expansion-evidence/`.

```bash
CGO_ENABLED=0 "$GO_HOST" build -o .local/gitversion-go ./cmd/gitversion-go
CGO_ENABLED=0 PARITY_DOTNET="$DOTNET_HOST" "$GO_HOST" test \
  -tags expansion -run '^TestGeneratedCalculationParity$' -count=1 -timeout 30m -v ./cmd/parity

export PARITY_PUBLIC_SOURCES='{"GitTools/actions":"/absolute/local/actions","GitTools/build-images":"/absolute/local/build-images"}'
CGO_ENABLED=0 PARITY_DOTNET="$DOTNET_HOST" "$GO_HOST" test \
  -tags expansion -run '^TestPublicHistoryReplayAndRepeatability$' -count=1 -timeout 20m -v ./cmd/parity
```

Public replay uses the manifest's real Git objects, explicitly selected tags, projected local branch refs, and a synthetic frozen configuration ignored through `.git/info/exclude`. It does not claim to reproduce either project's actual release configuration. Sources are read locally; no fetch or clone is performed. Existing project configuration is rejected. Run the public test with other experiment jobs idle when collecting benchmark evidence. It includes 20 verified, interleaved startup measurements per calculator on two cases; use `scripts/summarize-benchmark.py --help` to summarize retained samples.

The `platform` test tag exposes `TestPlatformCuratedParity`, which verifies unchanged published C# assemblies, records the platform-specific runtime/host hashes, executes all 63 curated cases, and checks the original 41 against their portable reference. Set absolute `PARITY_DOTNET` and `PARITY_CANDIDATE` paths. It requires runtime 10.0.14, not an automatically rolled-forward version.

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 "$GO_HOST" build -o .local/platform-bin/linux-arm64/gitversion-go ./cmd/gitversion-go
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 "$GO_HOST" test -c -tags platform -o .local/platform-bin/linux-arm64/parity.test ./cmd/parity
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 "$GO_HOST" build -o .local/platform-bin/windows-amd64/gitversion-go.exe ./cmd/gitversion-go
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 "$GO_HOST" test -c -tags platform -o .local/platform-bin/windows-amd64/parity.test.exe ./cmd/parity
```

Cross-builds only establish compilation. On Windows, copy the module including the published `.local/baseline/` and original lock, preserve the layout, and run `scripts/run-platform.ps1 -Dotnet C:\absolute\path\dotnet.exe`. This is prepared but has not been executed. The Linux execution command and image digest are recorded in the D report; all containers and artifacts remain local.

### Original C# test ports

[Calculation and configuration ports](CSHARP-PORTS.md) retain original assertions,
parameter cases and source hashes. The mapping inventory now covers 678 of 2,178
C# declarations; remaining source tests are explicitly pending.

[Semantic-version unit ports](CSHARP-SEMVER-PORTS.md) add all 1,158 original cases
from 14 methods. The active [90% unit-test goal](UNIT-TEST-PORTING.md) tracks
640 verified unit/component methods out of 1,516 separately from integration ports.

[Parsing, formatting and regex ports](CSHARP-MODEL-PATTERNS-PORTS.md) add 42 methods
and 171 original cases, verified on both priority platforms. All 21 methods in
`SemanticVersionTests.cs` and all 35 in `RegexPatternTests.cs` are now mapped.

[Label tokenization and input validation](CSHARP-LABEL-VALIDATION-PORTS.md) adds
24 complete methods / 82 original cases as prerequisites for configuration-label
resolution. Both priority platforms are verified.

[Placeholder and formatter ports](CSHARP-TEMPLATE-FORMATTING-PORTS.md) add 50
complete methods / 89 original cases, including missing/null distinctions,
recursive object lookup, environment fallbacks and invariant formatting.

[Formatter dispatch and configuration labels](CSHARP-DISPATCH-LABELS-PORTS.md)
add 33 methods / 64 original cases. Global `calculation.mode` now supplies the
main/support default while preserving workflow-specific and explicit branch modes.
[Mainline and repository compatibility](CSHARP-MAINLINE-PORTS.md) adds original
Mainline cases, common Git configuration access, remote/private ref handling,
strict branch version extraction, metadata tags and corrected ignore traversal.
Passing these gates does not establish full replacement compatibility.

[Merge-message source ports](CSHARP-MERGE-MESSAGE-PORTS.md) add all 15 original
methods / 49 cases and connect the parser to version calculation.

[Version-source filter ports](CSHARP-VERSION-FILTER-PORTS.md) add all 12 original
filter methods. Reusable filter APIs are verified; SHA/date/path ignore wiring is now available.

[Version-source metadata and ignore wiring](CSHARP-VERSION-SOURCE-PORTS.md) add all
19 original methods / 27 cases and 32 fresh ignore combinations on both OS.

[Ignore configuration and selected legacy loading](CSHARP-IGNORE-CONFIGURATION-PORTS.md)
add 21 original methods / 35 cases, branch/tag filters, remote source discovery and
a corrected date-floor boundary. V6 selection supports the declared fields.

[Configuration file locator ports](CSHARP-FILE-LOCATOR-PORTS.md) add all 14 original
methods / 39 cases, default configuration loading and explicit `--config` paths.

[Configuration document mapper ports](CSHARP-DOCUMENT-MAPPER-PORTS.md) cover all
10 original methods and 19 cases, including property ownership and legacy nesting.
