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

Platform verification is pending; no new methods count as verified yet.
