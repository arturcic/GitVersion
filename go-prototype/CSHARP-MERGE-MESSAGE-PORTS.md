# Merge-message source ports

Pinned C# baseline: `14d222235b7dabb7033473528a76fd18478a9217`.
All 15 MergeMessageTests methods and 49 cases retain original assertions for
format name, target/source branch, pull-request number/flag, semantic version,
null/empty values and null-argument errors. Builder dictionaries become ordered
Go format lists. EmptyTagPrefix retains all three original arguments: its message
matches no pattern, so the prefix does not participate in a parse.

The production calculator now uses the shared parser for Default, SmartGit,
Bitbucket Server/Cloud/v7, GitHub, remote-tracking and Azure DevOps messages.
Configured patterns run first in order. Named captures distinguish absence from
an empty match. Integer parsing uses the C# Int32 range. Regexes retain original
case options, and version extraction uses the existing strict branch parser.

| Requirement | Evidence |
| --- | --- |
| All original format, capture, null and precedence assertions | `TestCSharpMergeMessages` |
| Version output through eight built-in merge formats | `TestPlatformRepositoryCompatibilityParity` |

Native and Ubuntu gates are pending; mappings remain unverified until both pass.
Full replacement, configurable CLI merge formats and Windows remain pending.
