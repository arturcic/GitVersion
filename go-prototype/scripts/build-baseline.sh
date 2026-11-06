#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
dotnet_host="${DOTNET_HOST:-$(command -v dotnet)}"
go_host="${GO_HOST:-$(command -v go)}"
module_root="$PWD"
repo_root="$(git rev-parse --show-toplevel)"
revision="$(python3 -c 'import json; print(json.load(open("baseline.json"))["revision"])')"
sdk="$(python3 -c 'import json; print(json.load(open("baseline.json"))["sdk"])')"
project="$(python3 -c 'import json; print(json.load(open("baseline.json"))["project"])')"
framework="$(python3 -c 'import json; print(json.load(open("baseline.json"))["framework"])')"
configuration="$(python3 -c 'import json; print(json.load(open("baseline.json"))["configuration"])')"
if [[ -e .local/baseline.lock.json || -e .local/source || -e .local/baseline ]]; then
  echo 'Baseline paths already exist; preserve them or use a fresh checkout.' >&2
  exit 1
fi
mkdir -p .local/source
git -C "$repo_root" archive --format=tar "$revision" > .local/source.tar
tar -xf .local/source.tar -C .local/source
(
  cd .local/source
  [[ "$("$dotnet_host" --version)" == "$sdk" ]]
  "$dotnet_host" publish "$project" --configuration "$configuration" --framework "$framework" --output "$module_root/.local/baseline" --nologo
)
CGO_ENABLED=0 "$go_host" run ./cmd/parity pin --dotnet "$dotnet_host"
