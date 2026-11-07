# Configuration migration and version selection

Checkpoint `5a9413396` ports all 12 migration service methods, both executor
methods, 17 configuration version methods, and five parser methods. The 36
methods cover 60 original cases.

`gitversion-go config migrate` prints migrated YAML by default. It accepts
`--config PATH`, `--output PATH`, `--force` with an output path, and
`--in-place`. The command validates before writing, preserves configured
values, and leaves effective defaults out of the migrated document. Comments
are discarded. Replacement writes use a temporary sibling and an atomic
rename. New output uses an atomic link so a concurrent writer cannot be
overwritten. Both paths clean up their temporary file.

The calculator resolves `{Sha}` and `{ShortSha}` from the relevant commit.
Six fresh C# comparisons cover both labels in flat, nested, and migrated
configuration. The CLI also emits the v6 compatibility warning for a
successfully loaded legacy file and keeps it on stderr.

macOS arm64 and Ubuntu 26.04.1 arm64 passed 38,341 Go tests and subtests in
11 packages. Fifteen fresh migration comparisons per OS matched exit status,
stdout, and the final bytes of all files. The tests compare required diagnostic
text because C# prefixes some warnings with timestamps. Each platform passed
all 110 repository and configuration, 88 global mode, 87 calculation,
19 query and display, 17 validation, and six configuration value comparisons.
The actual checkout returned `7.0.0-alpha.1+739` with all 33 fields and JSON
types matching the pinned C# build.

`CSHARP-CONFIGURATION-MIGRATION-EVIDENCE.json` binds the source, executables,
test logs, and offline Linux inputs. The 90% target has 608 unit and component
methods remaining. Windows validation stays last.
