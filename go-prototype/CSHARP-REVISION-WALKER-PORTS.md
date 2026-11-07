# Git revision walker contracts

Source checkpoint `b10c6c144` ports 16 original C# methods and all 53 cases.
The six history matrices retain all seven sort combinations. The fixtures
cover linear, merged, equal-time, criss-cross, and octopus histories, with
exclusions, first-parent walks, and multiple tips. Merge-base checks include
both directions of the criss-cross graph, identity, and unrelated roots.

The original C# LibGit2Sharp expressions supplied 48 ordered SHA or merge-base
results across 47 cases. The Go suite compares those results and preserves the
original direct assertions in the remaining six cases. The shallow fixtures
encode the same boundary and commit availability as the original depth-two
clone and the graft over present parents. The Go storage layer honors these
grafts. The calculation entry point still requires a full clone.

macOS arm64 and offline Ubuntu 26.04 arm64 each passed 39,355 regular tests
and subtests in 13 packages. Both platforms repeated all 48 oracle queries
and passed the 14 previous process comparison groups. On the actual checkout,
the Go and pinned C# executables produce `7.0.0-alpha.1+739` with all 33 JSON
values and types matching. Ubuntu uses a snapshot of the checkout.

The verified inventory reaches 1,093 of 1,516 unit and component methods
(72.10%). Another 272 methods are needed for 90%. Windows validation is pending.

`CSHARP-REVISION-WALKER-EVIDENCE.json` records the source and executable hashes,
case counts, process reports, and checkout snapshot.
