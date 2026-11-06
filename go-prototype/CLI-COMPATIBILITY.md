# Pinned CLI compatibility inventory

Generated from the immutable baseline lock’s captured `--cli-schema`, not from a moving website. Reference SHA: `14d222235b7dabb7033473528a76fd18478a9217`. The Go prototype currently exposes separate calculate/observe commands; these C# CLI contracts remain open. Matching an option name is insufficient: arguments, aliases, precedence, stdout/stderr, exit status and side effects need differential tests.

| Command | Option | Aliases | Status |
| --- | --- | --- | --- |
| `gitversion` | `--help` | `-?, -h, /?, /h` | Open |
| `gitversion` | `--version` | — | Open |
| `gitversion` | `--diagnose` | `-d` | Open |
| `gitversion` | `--log-file` | `-l` | Open |
| `gitversion` | `--output` | `-o` | Open |
| `gitversion` | `--output-file` | — | Open |
| `gitversion` | `--show-variable` | `-v` | Open |
| `gitversion` | `--format` | `-f` | Open |
| `gitversion` | `--config` | `-c` | Open |
| `gitversion` | `--show-config` | — | Open |
| `gitversion` | `--override-config` | — | Open |
| `gitversion` | `--target-path` | — | Open |
| `gitversion` | `--no-fetch` | — | Open |
| `gitversion` | `--no-cache` | — | Open |
| `gitversion` | `--no-normalize` | — | Open |
| `gitversion` | `--allow-shallow` | — | Open |
| `gitversion` | `--verbosity` | — | Open |
| `gitversion` | `--update-assembly-info` | — | Open |
| `gitversion` | `--update-project-files` | — | Open |
| `gitversion` | `--ensure-assembly-info` | — | Open |
| `gitversion` | `--update-wix-version-file` | — | Open |
| `gitversion` | `--url` | — | Open |
| `gitversion` | `--branch` | `-b` | Open |
| `gitversion` | `--username` | `-u` | Open |
| `gitversion` | `--password` | `-p` | Open |
| `gitversion` | `--commit` | — | Open |
| `gitversion` | `--dynamic-repo-location` | — | Open |
| `gitversion` | `--cli-schema` | — | Open |
| `gitversion` | `--telemetry-opt-out` | — | Open |
| `gitversion config migrate` | `--config` | `-c` | Open |
| `gitversion config migrate` | `--output` | — | Open |
| `gitversion config migrate` | `--in-place` | — | Open |
| `gitversion config migrate` | `--force` | — | Open |
| `gitversion config get` | `--config` | `-c` | Open |
| `gitversion config get` | `--override-config` | — | Open |
| `gitversion config get` | `--target-path` | — | Open |
| `gitversion config get` | `--log-file` | `-l` | Open |
| `gitversion config get` | `--verbosity` | — | Open |
| `gitversion config validate` | `--config` | `-c` | Open |
| `gitversion config validate` | `--override-config` | — | Open |
| `gitversion config validate` | `--target-path` | — | Open |
| `gitversion config validate` | `--format` | — | Open |
