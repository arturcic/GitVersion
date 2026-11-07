# Effective branch configuration and reference names

Implementation checkpoints `ea3f4e5b8` and `d8bd76cea` preserve nullable settings
through every branch in a source chain. Explicit values win, including None,
false, zero weight and empty labels. The global increment applies when an
inheriting branch has no source. An inherited increment with no global fallback
still produces no effective configuration. A shared traversal set prevents
cycles. The single-branch settings accessor still exposes the resolved default
mode; the recursive finder reads declared settings so an unset mode can inherit.

A pull-request message can select an allowed target before source inheritance
continues. A local matching branch wins over a remote match; an origin-only
match is accepted. Upstream names keep their remote prefix when matching a
friendly target. The pinned C# release-branch rule also recognizes a release
message on a single-parent commit, as in a squash merge. Missing or disallowed
targets leave source candidates intact. Fetched build-agent context remains
outside this implementation.

All **17** original `EffectiveBranchConfigurationFinderTests` methods retain
**63** enum and explicit parameter cases. The Go source callback preserves each
original mock graph and its branch identities, cardinality and order. All **6**
`ReferenceNameTests` methods retain **18** cases, including literal tag names,
origin versus upstream, PR namespaces, and null versus empty tag arguments.
The production reference type is shared by configuration and target matching.

macOS arm64 and offline Ubuntu 26.04.1 arm64 passed **39,149** regular tests and
subtests in 13 packages, without failures or skips. The same test names were
observed on both platforms. The old parser-only main-Inherit rejection was
replaced by invalid-enum rejection and a live valid main-Inherit case. All
previous process comparison groups passed. Twenty-two new histories match the
pinned C# executable across all 33 output values and types, covering recursive
settings, global fallback, target precedence, and release squash messages.

The previous Go binary calculated `1.1.0-PullRequest2.3` when the merge message
named a support target; the verified version is `1.0.2-PullRequest2.3`. It also
lost an intermediate ManualDeployment mode and rejected `calculation.increment`.
Retained before/after reports exercise all three differences. The actual checkout
still returns `7.0.0-alpha.1+739`, with all 33 fields equal on both platforms.

The total is **1,016 / 1,516 unit and component methods (67.02%)**. The 90%
target needs **349** more verified methods. Additional calculation and
configuration cases, repository preparation, CLI integrations, and Windows
remain pending. `CSHARP-BRANCH-INHERITANCE-EVIDENCE.json` ties the source and enum
blobs, expanded original cases, test logs, binaries, process reports, and
network-isolated Ubuntu archives to the corrected implementation checkpoint.
