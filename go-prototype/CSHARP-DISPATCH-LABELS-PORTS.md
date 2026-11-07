# C# formatter dispatch and configuration-label ports

Implementation: `9e43ced31`. Global deployment-mode correction: `209a145bd`.
C# baseline: `14d222235b7dabb7033473528a76fd18478a9217`.

This batch preserves 33 original methods and 64 original cases:

| Source class | Methods | Cases | Exact Go test |
| --- | ---: | ---: | --- |
| FormattableFormatterTests | 5 | 12 | `TestCSharpDateAndFormattableFormatters` |
| DateFormatterTests | 3 | 4 | `TestCSharpDateAndFormattableFormatters` |
| ValueFormatterTests | 9 | 9 | `TestCSharpValueFormatterDispatch` |
| ConfigurationExtensionsTests (label methods only) | 16 | 39 | `TestCSharpConfigurationLabels` |

The source extractor reads immutable Git blobs and records source/case hashes.
Formatter cases retain expected values, handled flags, priorities and registry
state transitions. Label cases retain the original strings and assertions;
the source builder/mock setup is projected to effective label/regex values,
an injected environment lookup, and explicit SHA/ShortSHA values. Two unrelated
ConfigurationExtensionsTests methods remain pending and are not counted.

C# decimal values map to an exact decimal string backed by rational arithmetic.
Date values preserve the UTC/unspecified distinction. Formatter removal uses a
Go type exemplar in place of C# generic type syntax. Go errors replace exceptions;
explicitly asserted diagnostic text remains preserved. This is a source port,
not a claim that this harness executes the original NUnit unit suite.

| Requirement | Evidence |
| --- | --- |
| Original date and formattable results, flags and priorities | `TestCSharpDateAndFormattableFormatters` |
| Register/remove behavior and dispatch order | `TestCSharpValueFormatterDispatch` |
| Original label/null/override/environment/SHA contracts | `TestCSharpConfigurationLabels` |
| Stable priority order, typed null, numeric bounds and exact decimal rounding | `TestFormatterRegistryAndNumericBoundaries` |
| Invariant dates, invalid formats and template composition | `TestDateFormattingAndTemplateDispatch` |
| Named captures, UTF-16 sanitization and missing context | `TestLabelResolutionBoundaries` |
| Global deployment-mode inheritance and branch overrides | `TestGlobalDeploymentModePrecedence`, `TestGlobalDeploymentModeValidation` |
| Full output parity for global modes and the reported repository configuration | `TestPlatformGlobalDeploymentModeParity` |

Global `calculation.mode` supplies the workflow default for main and support.
Other branch types have explicit workflow modes. Explicit branch configuration
wins over the global value irrespective of YAML key order. The fresh executable
matrix has 44 histories / 88 checkpoints across both workflows and all three
modes, including the reported `.GitVersion.yml` on deterministic histories.

Remaining limits: full culture-specific formatting/parsing, all custom numeric
format syntax, and calculator integration of custom label expressions. The actual
GitVersion checkout still fails in go-git on `extensions.worktreeconfig`; its Git
configuration was not modified. Remote-reference handling and arbitrary local
branch names also remain separate repository-compatibility work. Passing this
matrix does not establish full compatibility on the user's real checkout.

macOS arm64 and Ubuntu 26.04.1 arm64 both passed 5,059 regular tests/subtests
in 11 packages with zero failures/skips. Each also passed fresh 87 existing
calculation JSON comparisons, six configuration checks, and all 88 new global-mode
JSON comparisons. `CSHARP-DISPATCH-LABELS-EVIDENCE.json` records production source,
binary and report hashes. The Ubuntu container used no network or host mounts;
its logs and evidence were exported before removal.

Verified unit/component ports: **169 / 1,516 (11.15%)**. The 90% target remains
1,365, with **1,196 more** required. Integration methods remain 38 / 662; overall
mapping is 207 / 2,178 (9.50%). These percentages measure source-test ports, not
statement coverage or full product compatibility. Everything stays local;
Windows stays last.
