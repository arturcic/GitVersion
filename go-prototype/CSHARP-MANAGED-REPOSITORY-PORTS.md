# Managed repository layout, config, and object IDs

Checkpoints `0b3bc4193` and `fa48015d4` add an exact layout reader and a separate discovery
operation. A linked worktree uses its own HEAD and index with the refs, objects,
and config from its common directory. The existing calculation and observation
commands use this layout through go-git. Bare and shallow layouts can be inspected
at the storage layer. Calculation still requires a complete, clean working tree
and an attached local branch.

The port preserves all 17 object ID methods with 23 cases, eight layout methods,
and six config file methods. The object ID tests preserve SHA-1 and SHA-256
widths, equality, dictionary lookup, raw bytes, and prefix boundaries. The config
tests use the same go-git decoder used to read repository config. They retain
the original quoted and unquoted path, comment, escape, and whitespace values.

The layout tests assemble local histories and clone layouts with go-git.
The first Ubuntu run exposed a Git executable dependency in the local clone
transport. The corrected fixtures populate the clone stores directly. Linked worktree
fixtures reproduce Git's HEAD, index, and directory pointers. SHA-256 and
reftable fixtures reproduce the format markers emitted by Git. This keeps the
original rejection cases runnable in the offline Ubuntu image without a Git
executable. The object ID type accepts SHA-256 values while the repository
reader rejects the SHA-256 storage format.

macOS arm64 and offline Ubuntu 26.04.1 arm64 passed 39,221 regular tests and
subtests in 13 packages. Four live linked worktree histories match all 33 C#
JSON fields, including each worktree's branch and SHA. The checks compare
independent copies and verify that neither the worktree nor its common storage
changes. The prior binary rejected the linked worktree input. The updated
binary matches the C# result on the same fixture. All 12 previous process
comparison groups passed on both platforms.

The user's GitVersion checkout still produces `7.0.0-alpha.1+739` with all 33
fields matching the pinned C# executable. The verified inventory is 1,047 of
1,516 unit and component methods (69.06%). The 90% target requires another 318
methods. Windows validation remains pending.

`CSHARP-MANAGED-REPOSITORY-EVIDENCE.json` records the checkpoint, source hashes,
case identities, binaries, process results, and offline Ubuntu archives.
