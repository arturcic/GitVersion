# Configuration validation command contracts

Source checkpoint `2b59a9e93` ports all 16 methods and 62 cases from the pinned
C# configuration validation suite. The Go port exercises normal CLI dispatch,
diagnostic codes and JSON types, stderr behavior, input redaction, path and
workflow selection, v6 compatibility, and read-only repository behavior.

The Go command recognizes unsupported root options before validation and
produces the same invocation diagnostic as C#. Options in the wrong command
scope still fail during parsing. Explicit file validation bypasses discovery.
The service-isolation test uses the existing Go command boundary, a discovery
callback that fails on use, and malformed Git metadata. Both original backend
environment values exercise the same pure-Go command.

Thirteen original invocation cases match the pinned C# executable on macOS
and offline Ubuntu 26.04, including exit status, exact stderr, and all JSON
values and types or empty stdout. Malformed YAML cases retain the original
positive source-line assertion. SharpYaml and Go YAML can report different
line and column positions for the same malformed document. Under the legacy
argument parser, both commands exit with status 1 and empty stdout. Their
stderr messages differ.

Each platform passed 39,781 regular tests and subtests across 13 packages, as
well as the previous 16 process comparison groups. The actual checkout still
returns `7.0.0-alpha.1+739` with all 33 values and types matching C#. Ubuntu
uses a snapshot of that checkout.

The verified inventory reaches 1,122 of 1,516 unit and component methods
(74.01%). Another 243 methods are needed for 90%. Windows validation is pending.

`CSHARP-CONFIG-VALIDATION-EVIDENCE.json` records the source and executable
hashes, case counts, process reports, and checkout snapshot.
