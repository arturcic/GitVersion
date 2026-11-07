# Configuration query and display

Checkpoint `467580907` ports all eight property-query methods, three
serializer methods, the provider's exact default YAML approval, and eleven
CLI methods. The 23 methods cover 82 original cases.

`gitversion-go config get calculation.tag-prefix` prints a single JSON scalar.
Queries use effective configuration and accept explicit files and runtime
overrides. The command distinguishes known nulls from unknown properties
and rejects dynamic collection paths. `gitversion-go --show-config`
prints effective YAML. Both commands use the existing configuration resolver
and run without a calculation. The exact default YAML approval matches byte
for byte.

macOS arm64 and Ubuntu 26.04.1 arm64 passed 38,238 Go tests and subtests
in 11 packages. Nineteen fresh C# query and display comparisons matched
stdout and stderr bytes and exit status per OS. Each confirmed unchanged
input files. The regular parity gates passed 104 repository and configuration,
88 global mode, 87 calculation, 17 validation, and six configuration value
comparisons per OS. All 33 values and JSON types still match on the actual
checkout, which returns `7.0.0-alpha.1+739`.

`CSHARP-CONFIGURATION-QUERY-EVIDENCE.json` binds source inputs, binaries, and
logs to the checkpoint. Verified coverage is 721 of 1,516 unit and component
methods, with 644 remaining to 90%.

The CLR alias and facade identity assertions and the C# dependency-injection
no-call test remain pending. The Go routing test independently exercises the command on an empty repository.
Windows validation stays last.
