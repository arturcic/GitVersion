# 90% C# unit-test porting target

User target: port 90% of the original C# unit tests to Go, keeping all work local,
prioritizing calculation/configuration and validating macOS and Ubuntu 26.
Windows validation stays last.

The pinned static inventory has **1,516 unit-or-component test declarations**
outside the explicit IntegrationTests directories and **662 integration declarations**.
Using this conservative existing classification, the target is **at least 1,365
fully mapped and verified unit/component declarations**. The first batch added
six unit methods plus 38 integration methods; the increment batch added 14 unit
methods and the parsing/formatting/regex batch added 42 more. Label tokenization and input validation added another 24. Placeholder/formatter ports added 50; formatter dispatch and configuration labels added 33; Mainline scenarios added 380 methods / 32,064 original cases. Merge-message parsing adds 15 methods / 49 cases. Version-source filter APIs add 12 methods / 12 cases. Version-source metadata and operator tests add 19 methods / 27 cases. Ignore configuration and version selection add 21 methods / 35 cases. Configuration file lookup adds 14 methods / 39 cases. Document mapping adds 10 methods and 19 cases. The variable provider adds 18 methods and 21 cases. Configuration resolution adds 40 methods and 78 cases. Configuration query and display add 23 methods and 82 cases. Migration and version selection add 36 methods and 60 cases. POSIX and legacy argument parsing add 146 methods and 251 cases. Build-agent output adds 79 methods and 111 cases. Next-version calculation adds 11 complete real-history methods and 23 original assertion checkpoints; three mocked-context methods remain pending. Branch inheritance and reference names add 23 methods and 81 original cases. Managed repository layout, config, and object IDs add 31 methods and 37 original cases. Git references, status, and tree diffs add 30 methods and 30 cases. The revision walker adds 16 methods and 53 original cases. The version source strategies add 13 methods and 71 original cases. Configuration validation adds 16 methods and 62 original cases. The commit-log projection adds nine methods and their original history matrices. Git object readers add 23 methods and cases. Git index and stream contracts add 14 methods and 19 cases. Dual-backend repository parity adds 19 methods and their ordered matrices. Formatted CLI output and dotenv add six methods and 29 original cases. Assembly info updates add 19 methods and 55 original cases. The verified
unit total is **1,212 of 1,516 (79.95%)**, leaving **153** to reach the target.
Evidence: `CSHARP-PORTS-EVIDENCE.json`, `CSHARP-SEMVER-EVIDENCE.json` and
`CSHARP-MODEL-PATTERNS-EVIDENCE.json`, plus
`CSHARP-LABEL-VALIDATION-EVIDENCE.json` and
`CSHARP-TEMPLATE-FORMATTING-EVIDENCE.json` and
`CSHARP-DISPATCH-LABELS-EVIDENCE.json` and
`CSHARP-MAINLINE-EVIDENCE.json` and
`CSHARP-MERGE-MESSAGE-EVIDENCE.json` and
`CSHARP-VERSION-FILTER-EVIDENCE.json` and
`CSHARP-VERSION-SOURCE-EVIDENCE.json` and
`CSHARP-IGNORE-CONFIGURATION-EVIDENCE.json` and
`CSHARP-FILE-LOCATOR-EVIDENCE.json` and
`CSHARP-DOCUMENT-MAPPER-EVIDENCE.json` and
`CSHARP-VARIABLE-PROVIDER-EVIDENCE.json` and
`CSHARP-CONFIGURATION-RESOLVER-EVIDENCE.json` and
`CSHARP-CONFIGURATION-QUERY-EVIDENCE.json` and
`CSHARP-CONFIGURATION-MIGRATION-EVIDENCE.json` and
`CSHARP-ARGUMENT-PARSER-EVIDENCE.json` and
`CSHARP-BUILD-AGENT-EVIDENCE.json` and
`CSHARP-NEXT-VERSION-EVIDENCE.json` and
`CSHARP-BRANCH-INHERITANCE-EVIDENCE.json`,
`CSHARP-MANAGED-REPOSITORY-EVIDENCE.json`,
`CSHARP-REPOSITORY-CONTRACTS-EVIDENCE.json`,
`CSHARP-REVISION-WALKER-EVIDENCE.json`,
`CSHARP-VERSION-STRATEGIES-EVIDENCE.json`,
`CSHARP-CONFIG-VALIDATION-EVIDENCE.json`,
`CSHARP-COMMIT-LOG-EVIDENCE.json`,
`CSHARP-GIT-OBJECT-READERS-EVIDENCE.json`,
`CSHARP-GIT-INDEX-STREAMS-EVIDENCE.json`,
`CSHARP-DUAL-BACKEND-EVIDENCE.json`,
`CSHARP-FORMAT-ARGUMENTS-EVIDENCE.json`, and
`CSHARP-ASSEMBLY-INFO-EVIDENCE.json`.
The all-method mapping figure uses a different denominator.

The unsupported-extension assembly updater declaration remains pending. Its
mock expects a write for a missing JavaScript file, while the pinned updater
has no JavaScript template and its missing-template path returns without a
write. The Go inventory keeps this assertion separate from the 19 verified
methods.

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
