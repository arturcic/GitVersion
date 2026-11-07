# Pinned CLI option index

Reference SHA 14d222235b7dabb7033473528a76fd18478a9217. The surface comes from the pinned --cli-schema export. POSIX aliases are in scope. Slash aliases are shown for reference.

Use the [behavior checklist](PARITY-CHECKLIST.md) for acceptance criteria, exact test names and results.

| Command | Entry | Aliases | Mapped behavior and status |
| --- | --- | --- | --- |
| gitversion | command | — | [Partial · cli-parse](PARITY-CHECKLIST.md#cli-parse) |
| gitversion | path | — | [Partial · git-discovery](PARITY-CHECKLIST.md#git-discovery) |
| gitversion | --help | -?, -h, /?, /h | [Matched · cli-help](PARITY-CHECKLIST.md#cli-help) |
| gitversion | --version | — | [Matched · cli-version](PARITY-CHECKLIST.md#cli-version) |
| gitversion | --diagnose | -d | [Matched · cli-diagnose](PARITY-CHECKLIST.md#cli-diagnose) |
| gitversion | --log-file | -l | [Matched · cli-sinks](PARITY-CHECKLIST.md#cli-sinks)<br>[Partial · cli-traces](PARITY-CHECKLIST.md#cli-traces) |
| gitversion | --output | -o | [Partial · output-json](PARITY-CHECKLIST.md#output-json)<br>[Partial · output-text](PARITY-CHECKLIST.md#output-text)<br>[Partial · output-order](PARITY-CHECKLIST.md#output-order)<br>[Partial · ci-precedence](PARITY-CHECKLIST.md#ci-precedence) |
| gitversion | --output-file | — | [Partial · output-json](PARITY-CHECKLIST.md#output-json) |
| gitversion | --show-variable | -v | [Partial · output-text](PARITY-CHECKLIST.md#output-text) |
| gitversion | --format | -f | [Partial · output-text](PARITY-CHECKLIST.md#output-text) |
| gitversion | --config | -c | [Partial · cfg-lookup](PARITY-CHECKLIST.md#cfg-lookup)<br>[Partial · cfg-layout](PARITY-CHECKLIST.md#cfg-layout) |
| gitversion | --show-config | — | [Matched · cfg-display](PARITY-CHECKLIST.md#cfg-display)<br>[Partial · cfg-layout](PARITY-CHECKLIST.md#cfg-layout) |
| gitversion | --override-config | — | [Partial · cfg-override](PARITY-CHECKLIST.md#cfg-override) |
| gitversion | --target-path | — | [Partial · git-discovery](PARITY-CHECKLIST.md#git-discovery) |
| gitversion | --no-fetch | — | [Partial · git-normalize](PARITY-CHECKLIST.md#git-normalize)<br>[Partial · git-shallow](PARITY-CHECKLIST.md#git-shallow) |
| gitversion | --no-cache | — | [Partial · git-cache](PARITY-CHECKLIST.md#git-cache) |
| gitversion | --no-normalize | — | [Partial · git-normalize](PARITY-CHECKLIST.md#git-normalize) |
| gitversion | --allow-shallow | — | [Partial · git-shallow](PARITY-CHECKLIST.md#git-shallow) |
| gitversion | --verbosity | — | [Matched · cli-sinks](PARITY-CHECKLIST.md#cli-sinks)<br>[Partial · cli-traces](PARITY-CHECKLIST.md#cli-traces) |
| gitversion | --update-assembly-info | — | [Partial · output-assembly](PARITY-CHECKLIST.md#output-assembly) |
| gitversion | --update-project-files | — | [Partial · output-project](PARITY-CHECKLIST.md#output-project) |
| gitversion | --ensure-assembly-info | — | [Partial · output-assembly](PARITY-CHECKLIST.md#output-assembly) |
| gitversion | --update-wix-version-file | — | [Matched · output-wix](PARITY-CHECKLIST.md#output-wix)<br>[Open · output-wix-failures](PARITY-CHECKLIST.md#output-wix-failures) |
| gitversion | --url | — | [Partial · git-dynamic](PARITY-CHECKLIST.md#git-dynamic)<br>[Partial · git-auth](PARITY-CHECKLIST.md#git-auth) |
| gitversion | --branch | -b | [Partial · git-branch](PARITY-CHECKLIST.md#git-branch) |
| gitversion | --username | -u | [Partial · git-auth](PARITY-CHECKLIST.md#git-auth) |
| gitversion | --password | -p | [Partial · git-auth](PARITY-CHECKLIST.md#git-auth) |
| gitversion | --commit | — | [Partial · git-branch](PARITY-CHECKLIST.md#git-branch)<br>[Partial · git-cache](PARITY-CHECKLIST.md#git-cache) |
| gitversion | --dynamic-repo-location | — | [Partial · git-dynamic](PARITY-CHECKLIST.md#git-dynamic)<br>[Decision · git-clone-reuse](PARITY-CHECKLIST.md#git-clone-reuse) |
| gitversion | --cli-schema | — | [Matched · cli-schema](PARITY-CHECKLIST.md#cli-schema) |
| gitversion | --telemetry-opt-out | — | [Partial · cli-telemetry-opt-out](PARITY-CHECKLIST.md#cli-telemetry-opt-out) |
| gitversion config | command | — | [Partial · cli-parse](PARITY-CHECKLIST.md#cli-parse) |
| gitversion config migrate | command | — | [Partial · cfg-migrate](PARITY-CHECKLIST.md#cfg-migrate) |
| gitversion config migrate | --config | -c | [Partial · cfg-migrate](PARITY-CHECKLIST.md#cfg-migrate) |
| gitversion config migrate | --output | — | [Partial · cfg-migrate](PARITY-CHECKLIST.md#cfg-migrate) |
| gitversion config migrate | --in-place | — | [Partial · cfg-migrate](PARITY-CHECKLIST.md#cfg-migrate) |
| gitversion config migrate | --force | — | [Partial · cfg-migrate](PARITY-CHECKLIST.md#cfg-migrate) |
| gitversion config get | command | — | [Partial · cfg-query](PARITY-CHECKLIST.md#cfg-query) |
| gitversion config get | property-path | — | [Partial · cfg-query](PARITY-CHECKLIST.md#cfg-query) |
| gitversion config get | --config | -c | [Partial · cfg-query](PARITY-CHECKLIST.md#cfg-query)<br>[Partial · cfg-lookup](PARITY-CHECKLIST.md#cfg-lookup) |
| gitversion config get | --override-config | — | [Partial · cfg-query](PARITY-CHECKLIST.md#cfg-query)<br>[Partial · cfg-override](PARITY-CHECKLIST.md#cfg-override) |
| gitversion config get | --target-path | — | [Partial · cfg-query](PARITY-CHECKLIST.md#cfg-query)<br>[Partial · cfg-lookup](PARITY-CHECKLIST.md#cfg-lookup) |
| gitversion config get | --log-file | -l | [Partial · cfg-query](PARITY-CHECKLIST.md#cfg-query)<br>[Matched · cli-sinks](PARITY-CHECKLIST.md#cli-sinks) |
| gitversion config get | --verbosity | — | [Partial · cfg-query](PARITY-CHECKLIST.md#cfg-query)<br>[Matched · cli-sinks](PARITY-CHECKLIST.md#cli-sinks) |
| gitversion config validate | command | — | [Partial · cfg-validate](PARITY-CHECKLIST.md#cfg-validate) |
| gitversion config validate | --config | -c | [Partial · cfg-validate](PARITY-CHECKLIST.md#cfg-validate)<br>[Partial · cfg-lookup](PARITY-CHECKLIST.md#cfg-lookup) |
| gitversion config validate | --override-config | — | [Partial · cfg-validate](PARITY-CHECKLIST.md#cfg-validate)<br>[Partial · cfg-override](PARITY-CHECKLIST.md#cfg-override) |
| gitversion config validate | --target-path | — | [Partial · cfg-validate](PARITY-CHECKLIST.md#cfg-validate)<br>[Partial · cfg-lookup](PARITY-CHECKLIST.md#cfg-lookup) |
| gitversion config validate | --format | — | [Partial · cfg-validate](PARITY-CHECKLIST.md#cfg-validate) |
