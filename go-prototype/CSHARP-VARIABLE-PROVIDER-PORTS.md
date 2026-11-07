# Version variable provider

Checkpoint `112a3be66` ports all 18 methods and 21 cases from the pinned
`VariableProviderTests`. The Go tests compare all 33 fields and JSON types
in each of the eight original approvals. They also preserve the original
literal checks for release labels, custom formats, semantic and counting
sources, copies, and JSON round trips.

The Go CLI uses the same renderer. It accepts assembly schemes and assembly,
informational, custom version, and commit date formats through the existing
configuration loader. A branch custom format overrides the global format and
an empty branch format clears it. Legacy and nested settings use the same
output settings after document mapping.

macOS arm64 and Ubuntu 26.04.1 arm64 passed 38,006 Go tests and subtests in
11 packages. Fresh process checks passed 104 repository and configuration
cases, 88 global deployment mode cases, 87 calculation cases, and six
configuration value cases per OS. The actual checkout returned
`7.0.0-alpha.1+739` with all 33 JSON values and types equal on both platforms.

`CSHARP-VARIABLE-PROVIDER-EVIDENCE.json` records source and binary hashes,
the test logs, and the offline Linux inputs. The corpus records all original
approval hashes. Deterministic extraction, vet, and whitespace checks passed.
The target has 707 unit or component methods remaining. Windows validation
stays last.

Checkpoint `cc6c6f230` updates the prototype rejection test. It now checks
that an invalid format fails with a diagnostic and yields no version.
