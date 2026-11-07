# Commit log and increment calculation

Source checkpoint `8e3b7fb34` ports all nine methods in the pinned C#
repository commit-log suite. A deterministic go-git history retains both
merges and the original unmerged branch variant. The tests compare ordered
commit IDs against the independent revision walk for every base, every
excluded commit, and the full base-by-ignored matrix.

The Go calculator reuses the topological projection of one head while it
evaluates candidates. The tests count the actual walks and verify eviction
when the head changes. Base ancestry can cross ignored commits and enter the
projection from an unmerged branch. Tag pruning stops at ignored commits.
A separate regression check exercises the bounded walk used when a reset
makes ordering observable. The projection can break equal-time ties
differently, so the reset path retains the independent walk order.

macOS arm64 and offline Ubuntu 26.04 arm64 each passed 39,801 regular tests
and subtests in 13 packages. Both platforms matched 192 fresh C# reset-policy
calculations and the 17 previous process comparison groups. The actual
checkout still produces `7.0.0-alpha.1+739` with all 33 JSON values and types
matching. Ubuntu uses a snapshot of the checkout.

The verified inventory reaches 1,131 of 1,516 unit and component methods
(74.60%). Another 234 methods are needed for 90%. Windows validation is pending.

`CSHARP-COMMIT-LOG-EVIDENCE.json` records the source and executable hashes,
case counts, process reports, and checkout snapshot.
