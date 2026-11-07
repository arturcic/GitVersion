# Reusing the C# test suite

The source of truth is the pinned C# revision in `baseline.json`. The Go replacement
uses three complementary paths:

| Original test | Reuse in Go | What proves it |
| --- | --- | --- |
| Git-history integration scenario | Translate fixture setup into go-git recipes; retain named checkpoints and original assertions | Fresh C# and Go executions on independent copies; compare every JSON field and type |
| Unit test of a public rule | Port its input and expected behavior into Go table tests | Direct Go assertions for parsing, comparison, configuration inheritance and formatting |
| C# implementation-detail test | Design the equivalent Go behavior test | Test the Go public seam; no requirement to reproduce C# object structure |
| CLI / CI / MSBuild test | Reuse input/output contracts and consumer projects | Command behavior, exit codes, files, environment variables and consumer builds; pending |

`testdata/csharp-test-inventory.json` maps original test declarations to Go tests
and fixtures. Unmapped entries are explicitly pending. `scripts/inventory-csharp-tests.py`
reads immutable blobs from Git at the pin; it does not read the working C# tree.
This is a static inventory of attributed method declarations, **not** an executed
NUnit discovery count: parameter providers, inherited and generated cases still
need discovery and review. A mapped declaration is not a claim of complete class,
project or suite coverage.

The first direct ports cover six methods / seven cases from
`VersionInCurrentBranchNameScenarios` and `VersionInMergedBranchNameScenarios`.
`TestConfigurationEdgeFrozenReference` retains each original FullSemVer assertion
and checks all fields against an independently captured C# result. The corresponding
process check is `TestPlatformConfigurationEdgeParity`, now passing on macOS and Ubuntu. The six mapped declarations are marked `verified-macos-ubuntu`; those six are the first verified batch. The next 38 ports and their validation are described in [CSHARP-PORTS.md](CSHARP-PORTS.md), bringing explicit mappings to 44 declarations (2.02%); 2,134 remain pending. These counts do not measure compatibility.

The generated configuration and strategy matrices are additional tests. They do
not stand in for importing the existing suite. Future ports should add the source
method, exact parameter case, portable recipe and assertion before marking an
inventory entry verified. Windows validation remains a separate final gate.

The [semantic-version unit batch](CSHARP-SEMVER-PORTS.md) adds 14 original methods
with 1,158 literal parameter/assertion cases. Unit/component and integration totals
are tracked separately for the [90% unit-test target](UNIT-TEST-PORTING.md).
