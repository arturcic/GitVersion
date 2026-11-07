# Ignore configuration, legacy selection and source boundaries

Pinned C# baseline: `14d222235b7dabb7033473528a76fd18478a9217`.

All 18 IgnoreConfigurationTests methods retain their original v6 document strings
and assertions for dates, collections/defaults, empty state, builder values,
serialization property names, scalar strategy flags and diagnostic privacy.
A raw legacy document loader keeps values separate from effective calculation
validation, as in C#: loading next-version 1.0 is distinct from calculating with it.
The supported v6 fields can now be calculated when GITVERSION_CONFIGURATION_VERSION
selects v6. Selection is case-insensitive and trims whitespace. V7 remains default;
dialects stay explicit. Unsupported legacy fields fail rather than disappear.

Go records/empty slices replace nonnull configuration objects/HashSets; strategy
flag combinations become sets of enabled names. Errors replace the original
exception classes while retaining all explicitly asserted diagnostic substrings.
Date expectations preserve the original host-local wall-clock comparison.
The source-port harness does not execute NUnit.

Branch/tag patterns now exclude version sources with the pinned matching rules.
Canonical remote branch names retain their identity; origin prefixes are removed
for configuration matching, and any remote prefix is removed for ignore matching.
Remote HEAD aliases are excluded from branch iteration. Remote main branches
participate in Mainline traversal and inheritance. The current branch's history
is retained when branch-name patterns exclude other version sources.

The minimum-date boundary has two contracts: normal filtering keeps a commit
exactly at the minimum, while final alternative-tag floor selection excludes it.
Four new C# probes reproduced the previous incorrect 5.0.0 result and now match
0.0.1 where the pinned implementation requires it.

| Requirement | Evidence |
| --- | --- |
| Every original ignore configuration assertion | `TestCSharpIgnoreConfiguration` |
| Dialect selection, loader/effective distinction, sets and remote names | `TestLegacyDocumentAndSelectionBoundaries` |
| Full JSON parity for filters, date floor, remote sources and v6 selection | `TestPlatformRepositoryCompatibilityParity` |

Fresh repository probes total 91 checkpoints: previous 57, four date-floor,
16 tag-pattern, eight branch-pattern and six v6 cases. Every output value/type
is compared against an independent pinned C# process. Legacy probes override only
the configuration-version environment selection for both executables.

Three ConfigurationVersionSelectorTests methods / 17 original cases additionally
retain unset/empty/case/whitespace selection, explicit-v6 detection and all asserted
unknown-value diagnostics. `TestCSharpConfigurationVersionSelector` calls the same
selector used by configuration loading with an explicit nullable input.

Platform gates are pending. The 21 mappings remain unverified until macOS and
Ubuntu complete. Full legacy configuration/CLI coverage and Windows stay pending.
