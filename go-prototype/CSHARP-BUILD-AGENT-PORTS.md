# Build-agent output

Checkpoint `6f85742d4` ports all 79 original BuildAgents test methods and
111 cases across 15 source files. The adapters preserve environment detection,
branch and tag context, build-number formatting, variable prefixes, escaping,
and local integration files. Shared variable enumeration keeps file and
console output in the pinned order.

The root CLI now executes `--output buildserver` and `--output dotenv`.
Build-agent selection follows the pinned registration precedence. GitHub
Actions appends nonempty values to its environment file. GitLab, Jenkins,
CodeBuild, and Bitbucket produce the original integration files. The
`output.update-build-number` setting controls build-number messages.

macOS arm64 and Ubuntu 26.04.1 arm64 passed 38,959 Go tests and subtests in
13 packages. Twenty live C# comparisons per OS matched stdout bytes, all
generated integration files, and exit status. They cover adapter selection
precedence, missing GitHub environment files, disabled build-number updates,
and dotenv ordering. AppVeyor JSON is checked against the original literals,
and its HTTP request flow is tested with a local server.

All previous process parity gates also passed on both platforms. The actual
checkout still returns `7.0.0-alpha.1+739` with all 33 JSON values and types
matching the pinned C# build. Agent-driven repository normalization and
external AppVeyor execution remain unverified. Windows stays last.

`CSHARP-BUILD-AGENT-EVIDENCE.json` records source and binary hashes,
complete test logs, and the offline Ubuntu inputs. The 90% target has
383 unit and component methods remaining.
