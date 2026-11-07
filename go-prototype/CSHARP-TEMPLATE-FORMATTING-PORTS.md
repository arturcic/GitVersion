# C# placeholder and formatter test ports

Implementation checkpoints: `007c09ca5`, corrected by `4b427068c`.
Pinned C# revision: `14d222235b7dabb7033473528a76fd18478a9217`.

This batch retains **50 complete original methods and 89 original cases**:

| Original source class | Methods | Cases | Go test |
| --- | ---: | ---: | --- |
| StringFormatWithExtensionTests | 34 | 43 | `TestCSharpTemplateFormatting` |
| StringFormatterTests | 6 | 27 | `TestCSharpStringAndNumericFormatters` |
| NumericFormatterTests | 4 | 10 | `TestCSharpStringAndNumericFormatters` |
| EdgeCaseTests | 6 | 9 | `TestCSharpSanitizerEdgeCases` |

Extraction reads immutable Git blobs, preserves original values and assertions,
and verifies source/case-file hashes and complete method/case inventories. Flat
anonymous C# objects map to exact-key Go dictionaries. The semantic-version
fixture retains nested metadata in an exported-field Go object; flattening that
fixture would bypass the original recursive VersionSourceDistance lookup.
C# exceptions map to Go errors, with original explicit messages retained.

## Implementation and behavior

`FormatWith` uses an injected environment lookup, the previously ported tokenizer
and sanitizers, exact dictionary keys or exported object fields, and ordered
fallback evaluation. Missing values, present null values, empty strings and
literal fallbacks remain distinct. A failed earlier lookup is rethrown if no
later fallback produces a value. The complete expression is parsed before any
fallback is evaluated. Replacements do not recursively interpret replacement text.

String formatting adds invariant upper/lower/title/sentence/Pascal operations.
Numeric formatting adds the mapped invariant numeric cases, signed 32-bit
integer formatting and zero padding. Supplemental tests exercise signed binary/
hex formatting, grouping, scientific/currency/percent output and large precision.
Review removed an accidental 999-digit precision ceiling; the limit now follows
[.NET's documented numeric precision bound](https://learn.microsoft.com/dotnet/standard/base-types/standard-numeric-format-strings).

These ports do not finish all .NET formatting contracts. Culture-specific casing,
full custom numeric format syntax, date formatting, general formattable-object
and custom-formatter dispatch/registration remain open. Reflection is adapted to
Go exported fields rather than C# property getters. CLI custom-label integration
is a separate remaining step; this batch provides its production primitives.

## Requirement evidence

| Requirement | Evidence |
| --- | --- |
| Every original substitution/fallback assertion | `TestCSharpTemplateFormatting` |
| Original format results, handled booleans, priorities | `TestCSharpStringAndNumericFormatters` |
| Original Unicode and length boundaries | `TestCSharpSanitizerEdgeCases` |
| Missing/null/empty distinctions, eager parsing, environment short circuit, object lookup | `TestTemplateFallbackInteractions` |
| Numeric boundaries and string-format behavior | `TestNumericAndStringFormatterBoundaries` |
| Calculation and configuration regression | Full regular/frozen suite; fresh `TestPlatformCSharpPortsParity` and `TestPlatformCSharpConfigurationPorts` |

Original NUnit tests are not executed by this harness. Unit expected results
come from pinned C# source; fresh C#/Go process probes cover 87 calculation JSON
checkpoints and six configuration cases. Earlier full strategy-matrix results
remain historical evidence. The initial validation run predates the precision
correction; only final runs after `4b427068c` may verify this batch.

Both macOS arm64 and Ubuntu 26.04.1 arm64 passed **4,936 regular tests/subtests
in 11 packages**, zero failures/skips, plus fresh 87 JSON and six configuration
comparisons on each OS. `CSHARP-TEMPLATE-FORMATTING-EVIDENCE.json` binds exact
source inputs, reports, candidate binaries, logs and exported Linux archives.
The Ubuntu container ran without networking or host mounts and was removed after
evidence export.

Verified unit/component ports: **136 / 1,516 (8.97%)**. The 90% target remains
1,365, with **1,229 more** required. Integration methods remain 38 / 662; overall
mapping is 174 / 2,178. Everything stays local; Windows stays last.
