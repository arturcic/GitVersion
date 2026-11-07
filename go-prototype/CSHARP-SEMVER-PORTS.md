# C# semantic-version unit ports

Implementation checkpoint: `c4e567496`.
Source: `SemanticVersionTests.cs` at the pinned C# revision
`14d222235b7dabb7033473528a76fd18478a9217`.

This batch retains **14 complete original method declarations and 1,158 original
parameter/assertion cases**:

- Twelve increment methods: 96 literal NUnit cases each, covering stable, numeric
  prerelease, numbered labels and bare labels; None/Patch/Minor/Major; null, empty
  and explicit labels; Standard/Force/EnsureIntegrity modes.
- `ValidateInvalidVersionParsing`: all three original invalid input/prefix cases.
- `VersionSorting`: all three original ordering assertions.

The source extraction script reads immutable Git blobs and refuses unknown case
syntax or changed inventories. Expected version strings come directly from C#
attributes. `testdata/csharp-unit-port-mappings.json` binds source and data hashes,
exact methods, test names, every case ID and adaptation notes. Re-running the
extractor produces identical files.

`BumpWithMode` adds a typed increment mode and distinguishes nil (keep the current
label) from an explicit empty label (clear it). EnsureIntegrity increments an
existing prerelease number only if its core already expresses the requested bump;
otherwise it increments the requested core field. Existing calculator calls retain
their explicit-label Standard/Force behavior through `Bump`.

Every imported increment case also checks that the input version is unchanged.
`TestIncrementModeValidation` verifies invalid modes and increments reject without
mutation. Standard and forced behavior still run through the existing frozen
calculation corpus after this shared primitive change.

The unit expectations are copied from the original source; the original NUnit
runner is not executed by this port harness. Fresh C#/Go process comparisons still
cover the 87 calculation checkpoints and six configuration cases from the first
batch, on both platforms. Final evidence is in `CSHARP-SEMVER-EVIDENCE.json`.

This is historical evidence for the increment batch. The subsequent
[parsing, formatting and regex batch](CSHARP-MODEL-PATTERNS-PORTS.md) completes
the remaining seven methods in this source class, including loose formats,
custom prefixes, build metadata and large numeric ranges in the model API.
The calculator's input boundary remains unchanged.

The active target remains 90% of the pinned 1,516 unit/component declarations
(at least 1,365). At this historical checkpoint, both batches mapped 20 unit/component methods plus 38
integration methods; only successful macOS and Ubuntu gates mark ports verified.
Windows stays last and all work remains local.
