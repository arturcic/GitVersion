# C# label tokenization and input validation ports

Implementation checkpoint: `c492f2f61`.
Pinned source: `14d222235b7dabb7033473528a76fd18478a9217`.

This batch preserves **24 complete original methods and 82 original cases**:

| Source class | Methods | Cases | Go test |
| --- | ---: | ---: | --- |
| LabelTokenizerTests | 4 | 33 | `TestCSharpLabelTokens` |
| SanitizeFormatTests | 7 | 20 | `TestCSharpInputSanitizers` |
| SanitizeEnvVarNameTests | 6 | 13 | `TestCSharpInputSanitizers` |
| SanitizeMemberNameTests | 7 | 16 | `TestCSharpInputSanitizers` |

The deterministic extraction script reads immutable C# blobs. All parameter
values, expected token names and exact exception messages are retained. C#
exception assertions map to Go error returns; successful dotted-member cases
additionally assert the unchanged result. Data hashes and source hashes bind the
complete method mapping. Neither generated Go outputs nor inferred expectations
replace the original assertions.

`internal/formatting` adds production primitives for label expression tokenization
and input validation. Tokens retain property/environment/literal types and
optional format specifiers, with ordered `??` fallback handling. Quoting, escaped
quotes, malformed expressions, and 32-bit integer-literal classification match
the pinned source. The pin's single trailing `?` behavior is deliberately retained
and tested; it is not silently tightened to a different grammar.

Sanitizers preserve allowed names, length limits, control-character handling and
exact messages. Limits count UTF-16 code units. Null environment/member names
are represented by Go pointers. Name patterns use the pure-Go .NET-style pattern
wrapper; its previously documented Unicode casing boundary remains open.

These primitives are prerequisites for remaining configuration-label tests.
Environment/property resolution, formatter dispatch and full CLI custom-label
support are not implemented by this batch, and are not claimed as verified.

| Requirement | Evidence |
| --- | --- |
| Every original tokenizer case | `TestCSharpLabelTokens` |
| Every original sanitizer case and error message | `TestCSharpInputSanitizers` |
| Token type, nullable format and grammar boundaries | `TestLabelTokenKindsFormatsAndBoundaries` |
| Name and format length boundaries, null names, controls | `TestSanitizerBoundariesAndNullNames` |
| Calculation/configuration regression | Full regular/frozen suite; fresh `TestPlatformCSharpPortsParity` and `TestPlatformCSharpConfigurationPorts` |

The original NUnit runner is not invoked. Unit expectations come from the pinned
source; fresh C#/Go process comparisons cover 87 calculation JSON checkpoints
and six configuration cases. The prior complete strategy matrix remains
historical evidence.

Both macOS arm64 and Ubuntu 26.04.1 arm64 passed **4,780 regular tests/subtests
in 11 packages**, zero failures/skips, plus the fresh C# process probes above.
`CSHARP-LABEL-VALIDATION-EVIDENCE.json` binds the source checkpoint, reports,
candidates, logs and archived container evidence. Ubuntu executed without host
mounts or networking; its container was removed after evidence export.

Verified unit/component methods: **86 / 1,516 (5.67%)**. The 90% target remains
1,365, with **1,279 more** required. Integration methods remain 38 / 662; overall
mapping is 124 / 2,178. Everything stays local; Windows stays last.
