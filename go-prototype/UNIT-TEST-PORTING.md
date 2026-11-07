# 90% C# unit-test porting target

User target: port 90% of the original C# unit tests to Go, keeping all work local,
prioritizing calculation/configuration and validating macOS and Ubuntu 26.
Windows validation stays last.

The pinned static inventory has **1,516 unit-or-component test declarations**
outside the explicit IntegrationTests directories and **662 integration declarations**.
Using this conservative existing classification, the target is **at least 1,365
fully mapped and verified unit/component declarations**. The first batch added
six unit methods plus 38 integration methods; the increment batch added 14 unit
methods and the parsing/formatting/regex batch added 42 more. Current verified
unit total: **62 of 1,516 (4.09%)**, leaving **1,303** to reach the target.
Evidence: `CSHARP-PORTS-EVIDENCE.json`, `CSHARP-SEMVER-EVIDENCE.json` and
`CSHARP-MODEL-PATTERNS-EVIDENCE.json`.
The all-method mapping figure uses a different denominator.

This classification includes repository-backed Mainline/component tests outside
IntegrationTests. It is not NUnit runtime discovery. Do not silently exclude hard,
platform-specific or implementation-specific tests to improve the percentage.
Any later reclassification must be source-backed, documented and show both counts.

A method counts only when every original parameter case and behavior assertion
is retained or its language-specific adaptation is documented and reviewed, with
Go verification on both macOS and Ubuntu. A passing generated matrix does not
count as a port without a source mapping. Partial methods remain pending.

Priority order:

1. Semantic-version increment, comparison and parsing rules; configuration values,
   defaults, inheritance and validation.
2. Base-version selection, Mainline transitions, commit/tag/branch semantics and
   version variable formatting.
3. Managed Git operations and normalization, CLI contracts and integrations needed
   by the remaining source tests.
4. Windows execution as the final platform gate.

Track each batch in the source mapping manifests and dashboard, with local
checkpoint commits and independent C# expectations. The 90% goal remains active;
the first port batch is only a checkpoint toward it.
