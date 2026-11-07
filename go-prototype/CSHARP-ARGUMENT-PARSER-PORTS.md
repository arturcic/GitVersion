# POSIX and legacy argument parsing

Checkpoint `86c012c22` ports 80 POSIX parser methods, 63 legacy parser
methods, and all three quoted-string helper methods. These 146 methods
cover 251 original cases. The corpus records source hashes and extracts
original literal commands, parameter rows, and field and diagnostic
assertions. Go filesystem fixtures cover file discovery and assembly
file globs. Five parser methods already have migration ports, and five
remain pending for query binding and whole-object override equivalence.

The Go CLI accepts local C#-style invocation with an optional repository
path. It supports JSON and file output, a selected version variable,
custom formatting, and runtime overrides for supported calculation
settings. The original explicit calculation and observation commands
are still available. Set `GITVERSION_ARGUMENT_PARSER_VERSION=v6` for
legacy argument spellings. The configuration layout selector is independent.

The parser recognizes remote and branch targeting, build-server output,
and file updater options. Their runtime integrations remain pending.
Help and version output identify the Go prototype.

macOS arm64 and Ubuntu 26.04.1 arm64 passed 38,742 tests and subtests in
12 packages. Eighteen fresh root CLI comparisons per OS matched exit
status, JSON values and types, scalar output, output files, and expected
error text. Each platform also passed all 110 repository and configuration,
88 global mode, 87 calculation, 19 query, 17 validation, 15 migration, and
six configuration value comparisons. The actual checkout still returns
`7.0.0-alpha.1+739` with all 33 JSON values and types matching C#.

`CSHARP-ARGUMENT-PARSER-EVIDENCE.json` records source and binary hashes,
test logs, and offline Ubuntu inputs. The 90% target has 462 unit and
component methods remaining. Windows stays last.
