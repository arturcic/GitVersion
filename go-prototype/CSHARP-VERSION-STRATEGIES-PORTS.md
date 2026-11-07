# Version source strategy contracts

Source checkpoint `abbac9b35` ports 13 original C# methods and 71 cases.
The merge-message suite runs 259 original assertions, including all suffix
variations, true merges and squash messages, custom release classification,
and rejected branch names. The branch-name suite uses actual go-git refs
and reproduces the remote-tracking ref left by a fetch.

The configured next-version suite asserts exact candidate and incremented
versions, absent source, and safe validation diagnostics. The Go calculator
stores the applied version and increment rather than the C# operator object.
The tests verify the observable operator effects. Configured next-version
now uses the selected Strict or Loose format. This batch exercises that
format for configured values. Tag and branch-name format parity is tracked
separately. Remote release extraction uses the existing Git reference
normalizer so canonical origin refs work with hyphenated releases.

macOS arm64 and offline Ubuntu 26.04 arm64 each passed 39,702 regular tests
and subtests in 13 packages. The seven valid configured-version inputs match
all 33 JSON values and types in fresh two-process comparisons on each OS.
The prior 15 process comparison groups also pass. Both platforms reproduce
`7.0.0-alpha.1+739` and match all 33 fields on the actual checkout, using a
snapshot for Ubuntu.

The verified inventory reaches 1,106 of 1,516 unit and component methods
(72.96%). Another 259 methods are needed for 90%. Windows validation is pending.

`CSHARP-VERSION-STRATEGIES-EVIDENCE.json` records source and executable hashes,
case counts, process reports, and the checkout snapshot.
