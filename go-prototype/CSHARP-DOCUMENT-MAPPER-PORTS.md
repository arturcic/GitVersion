# Configuration document mapping

Checkpoint `eefc912a9` ports all 10 methods and 19 original cases from the
pinned C# `ConfigurationDocumentMapperTests`. The source corpus records the
pinned declarations and the C# schema properties. The Go tests also exercise
property ownership for all 37 root and 15 branch properties in that schema.

The production mapper validates property placement in nested v7 documents and
splits the flat v6 document into calculation and output sections. Branch
pre-release weights follow the same mapping. The Go calculator still applies
its own supported-field allowlist after layout validation.

Both macOS arm64 and Ubuntu 26.04.1 arm64 passed 37,966 Go tests and subtests
in 11 packages. Fresh comparisons with the pinned C# executable passed all
100 repository and configuration cases, 88 global deployment mode cases,
87 calculation cases, and six configuration value checks per platform.
The actual GitVersion checkout returned `7.0.0-alpha.1+739` with all 33 JSON
values and types equal on both platforms.

`CSHARP-DOCUMENT-MAPPER-EVIDENCE.json` records source and binary hashes, reports,
test logs and the offline Linux inputs. Go vet and deterministic extraction
checks passed. The 90% target has 725 unit or component methods remaining.
Full replacement coverage, including CLI and integrations, remains open.
Windows validation is deferred.
