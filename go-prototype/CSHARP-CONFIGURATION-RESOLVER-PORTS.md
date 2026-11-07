# Configuration provider and resolver

Checkpoint `ca445182b` ports 31 provider methods, all six resolver methods,
and all three override helper methods. The 40 methods cover 78 original
cases. The exact YAML serialization approval and CLR property-alias reflection
test remain pending. Six provider methods already had verified Go ports.

The resolver starts from the same GitFlow, GitHubFlow, and TrunkBased workflow
resources embedded by the pinned C# assembly. It validates file and runtime
inputs independently, merges them in precedence order, and resolves branch
relationships without a repository. The Go file provider also exercises the
one-time warning for explicit legacy loading.

`gitversion-go config validate --config PATH --format json` uses the resolver.
It accepts repeatable `--override-config KEY=VALUE` options for scalar fields
and reports typed diagnostics. The 17 live cases compare JSON fields and
types, stderr, exit status, and file integrity. Exact YAML syntax source
locations and remaining CLI behavior require further parity work.
TrunkBased is supported by configuration validation. Calculation still uses
its existing workflow and field support.

macOS arm64 and Ubuntu 26.04.1 arm64 passed 38,128 tests and subtests in
11 packages. Each platform also passed all 17 validation, 104 repository and
configuration, 88 global mode, 87 calculation, and six configuration value
comparisons. The actual checkout returned `7.0.0-alpha.1+739` with all 33
JSON values and types matching the pinned C# output.

`CSHARP-CONFIGURATION-RESOLVER-EVIDENCE.json` records the source, binaries,
test results, and offline Linux inputs. Deterministic extraction, vet, and
whitespace checks passed. The 90% target has 667 unit and component methods
remaining. Windows validation stays last.
