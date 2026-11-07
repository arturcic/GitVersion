# Next-version calculation and branch rules

Local implementation checkpoint `b42dbccc7` adds the `TrunkBased/preview1`
workflow to calculation, custom branch regex/source rules, first-match
selection with the unknown rule considered last, named capture substitution,
and nullable custom-branch mode/tag-control inheritance. Matching uses the
existing pure-Go .NET-style regex adapter. Frozen preset rules are shared;
user overrides remain local to each parsed configuration.

Eleven complete real-repository methods from the pinned
`NextVersionCalculatorTests.cs` preserve all 23 original FullSemVer assertions.
The fixture extractor reads the immutable C# source, preserves branches, tags,
parent order and every intermediate assertion checkpoint, and materializes the
C# builder-only default increment None for new custom branches. Default merges
in these original cases fast-forward; MergeTo and MergeNoFF remain true merges.
The three mocked-context methods remain pending and do not count toward progress.

macOS arm64 and offline Ubuntu 26.04.1 arm64 passed **39,035** regular tests and
subtests in 13 packages, with zero failures or skips. Every previous regular test
was still discovered. All 23 original checkpoints matched the live pinned C#
executable across every one of the 33 JSON values and types. Another 21 live
comparisons cover overlapping rules, default and explicit case sensitivity,
UTF-16 named captures, capture precedence over SHA placeholders, source rules,
inherited deployment and tagged-commit settings, and TrunkBased global modes.
All nine previous fresh process comparison groups also passed on each platform.

The actual GitVersion checkout still returns `7.0.0-alpha.1+739`, matching C# in
all 33 fields and JSON types on both platforms. Git objects, refs and config were
copied into the network-isolated Ubuntu container; the user's repository was
not modified. The previous local binary rejected custom branches and TrunkBased;
retained before/after output demonstrates that these cases exercise new behavior.

The unit/component total is **993 / 1,516 (65.50%)**, leaving **372** methods to
reach the 90% target. Full recursive effective-configuration inheritance, more
configuration combinations, remaining CLI/integration behavior and Windows
validation remain pending. The regex adapter's existing Unicode/culture limits
also remain documented. This bounded parity evidence does not establish complete
replacement compatibility.

`CSHARP-NEXT-VERSION-EVIDENCE.json` binds the source checkpoint, original source
and recipe hashes, candidate binaries, full test logs, fresh C# process reports,
and retained offline Ubuntu input/output archives.
