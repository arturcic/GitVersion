# Git reference, status, and tree diff contracts

Source checkpoint `3a13e09b6` ports 30 original methods and 30 cases.
The tests cover 12 reference store, 12 status calculator, and six tree diff
methods. They check loose and packed refs, peeled tags, symbolic refs, Git
index and worktree status, raw path ordering, initial trees, and first-parent
diffs. Calculation uses the same Go repository code paths.

The suite replays 19 independent results captured from Git and the pinned
LibGit2Sharp oracle. It repeats 16 LibGit2Sharp comparisons on each platform.
The Go executable and regular tests build with CGO disabled. The C# oracle
runs as a separate process.

macOS arm64 and offline Ubuntu 26.04 arm64 each passed 39,285 regular tests
and subtests across 13 packages. All 13 earlier process comparison groups
passed on both platforms. On the user's checkout, the Go and pinned C#
executables return `7.0.0-alpha.1+739` with all 33 JSON values and types
matching on both platforms. Linux uses a snapshot of the checkout.

The verified inventory reaches 1,077 of 1,516 unit and component methods
(71.04%). Another 288 methods are needed for 90%. Windows validation is pending.

`CSHARP-REPOSITORY-CONTRACTS-EVIDENCE.json` records source and executable
hashes, per-platform test counts, process reports, and the checkout snapshot.
