# Configuration file locator ports

C# baseline: `14d222235b7dabb7033473528a76fd18478a9217`.

All 14 declarations / 39 cases from the nested default/named locator classes
preserve filename priority, independent enumeration order, lowercase matching,
working/root ambiguity, custom absolute/relative/subdirectory paths, missing paths,
exact warning messages and default-provider log assertions. An injected filesystem
replaces the original filesystem mock; original empty files remain empty.
Fixture-relative paths replace temporary paths in exact diagnostic templates.

The two same-named ambiguity declarations are separate methods. Their static
identities now include pinned declaration lines; source_method retains the original
name. Other duplicate unported declarations receive the same treatment. All 2,178
IDs are unique; denominators remain 1,516 unit/component and 662 integration.
No previously verified declarations were affected.

Configuration loading and CLI now use the shared locator/provider. Missing or
empty configuration uses the pinned GitFlow defaults. `calculate --config PATH`
accepts explicit relative and absolute files; a missing explicit CLI path fails.
Standalone locator fallback remains distinct from explicit CLI path validation.
Unsupported configuration fields still fail explicitly. Windows stays pending.

| Requirement | Evidence |
| --- | --- |
| Every original locator/path/log/diagnostic assertion | `TestCSharpConfigurationFileLocator` |
| Real filesystem, outside paths, missing explicit path and provider boundaries | `TestNativeConfigurationLocatorBoundaries` |
| Complete JSON parity for defaults, file priority and config paths | `TestPlatformRepositoryCompatibilityParity` |

Seven new executable probes compare every C# output field/type for absent, empty,
empty-map, null-workflow and competing default files, plus relative/absolute custom
configuration. The full repository matrix has 98 checkpoints. These source ports
do not execute the original NUnit runner. Document mapping tests remain next.

Implementation checkpoints: `23432fe76` and `e3c11ea42`. Both macOS arm64 and
Ubuntu 26.04.1 arm64 passed **37,934 regular tests/subtests in 11 packages** with
no failures or skips. Fresh process checks passed 98 repository/configuration
JSON cases, 88 global-mode JSON cases, 87 calculation JSON cases and six
configuration checks per OS. All 33 actual-repository fields/types still match
C#: **7.0.0-alpha.1+739**.

`CSHARP-FILE-LOCATOR-EVIDENCE.json` binds source, binaries, corpus, reports, logs
and Linux inputs to e3c11ea42. The container used no network or host mounts.
Original case extraction is deterministic; vet and whitespace checks passed.

Verified unit/component total: **630 / 1,516 (41.56%)**; **735 more methods**
are required to reach 1,365 (90%). Integration remains 38 / 662; total mapping
is 668 / 2,178 (30.67%). Full document mapping, configuration provider behavior,
CLI/integration coverage and Windows remain pending.
