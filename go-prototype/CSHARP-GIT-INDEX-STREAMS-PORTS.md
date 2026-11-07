# Git index and stream contracts

Source checkpoint `5053c12a9` ports 14 original C# methods and 19 cases from
the index, delta, random-access, and zlib suites. Three captured Git
`ls-files --stage` expectations cover index versions 2, 3, and 4. The tests
also check the stat data, skip-worktree and intent-to-add flags, and empty
index behavior.

The Go status path reads the assume-unchanged bit omitted from go-git's
decoded index entries. On all three index versions, the supplemental test
confirms that the flag suppresses unstaged changes while preserving staged
changes. Three fresh C# comparisons per platform also match all 33 version
fields when the only changes are covered by the flag.

The delta tests use the buffered go-git decoder used by the default object
storage. The streaming API can surface a truncated copy as EOF. The Go
bounded section reader preserves independent offsets, EOF, and zero
allocations for byte-sized reads with a caller-owned buffer. The original
zlib bytes cover the smaller window and all three invalid headers.

macOS arm64 and offline Ubuntu 26.04 arm64 each passed 39,896 tests and
subtests across 13 packages. All 18 prior process comparison groups passed.
The actual checkout still produces `7.0.0-alpha.1+739` with all 33 JSON values
and types matching C#. Ubuntu uses a snapshot of the checkout.

The verified inventory reaches 1,168 of 1,516 unit and component methods
(77.04%). Another 197 methods are needed for 90%. Windows validation is pending.

`CSHARP-GIT-INDEX-STREAMS-EVIDENCE.json` records the source and executable hashes,
case counts, process reports, and checkout snapshot.
