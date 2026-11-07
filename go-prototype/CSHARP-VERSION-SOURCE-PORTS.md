# Version sources and ignore configuration

C# baseline: `14d222235b7dabb7033473528a76fd18478a9217`.

All 19 VersionSourceTests methods / 27 cases retain their original FullSemVer,
semantic source/version/increment, counting source/distance and legacy source
assertions. Deterministic go-git histories preserve parent order and branch/tag
operations. Original SHA variables bind to checkpoint identities. TrunkBased
builder settings are projected to explicit Mainline settings for these cases.
Operator assertions run through the production Mainline base/operator model;
the original mocked strategy supplies candidates to the shared strategy callback.

C# model-null legacy SHA maps to the empty Go trace string. That model assertion
is checked separately from the legacy JSON formatter's empty-string contract.
The source harness does not execute NUnit and does not generate expected versions
from the Go calculator.

`calculation.ignore` now accepts `sha`, `commits-before` and `paths`. SHA matching
uses case-insensitive prefixes; date filtering excludes earlier timestamps; path
filtering excludes a commit only when every changed path matches. Empty configured
lists do not create active filters. Rules apply to current-commit selection,
tag/source candidates, bump messages, Mainline nodes and commit counting. Graph
walks retain their original ancestry before filtering. Branch/tag name ignore
patterns remain explicitly unsupported in this prototype.

| Requirement | Evidence |
| --- | --- |
| Every original source/count/operator assertion | `TestCSharpVersionSources` |
| Typed ignore lists, dates, deduplication and validation diagnostics | `TestIgnoreConfigurationParsing` |
| Full version output across both workflows, Mainline, and ignore combinations | `TestPlatformRepositoryCompatibilityParity` |

The fresh process matrix adds 32 histories: two workflows × normal/Mainline ×
eight ignore settings. It covers full/prefix SHA exclusions, ignored bump messages,
date boundaries, ignored paths/current commits and empty rules. Configuration is
committed after the referenced SHA exists, using go-git and fixed metadata.
The 57-case repository matrix includes the previous 25 compatibility histories.

Platform gates are pending. All 19 source mappings remain unverified until the
full macOS and Ubuntu checks pass. Windows stays pending.
