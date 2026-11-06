#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
root="$PWD"
dotnet_host="${DOTNET_HOST:-dotnet}"
go_host="${GO_HOST:-go}"
test -f .local/baseline.lock.json
test -f .local/baseline/GitVersion.Git.Managed.dll
# Keep all generated probe output outside the checked-in source. No historical
# source or published baseline is rebuilt or altered by this step.
"$dotnet_host" build oracle/RepositoryObserver/RepositoryObserver.csproj \
  -c Release -o .local/observer --nologo \
  "-p:BaselineDirectory=$root/.local/baseline" \
  "-p:BaseIntermediateOutputPath=$root/.local/observer-obj/"
CGO_ENABLED=0 "$go_host" build -o .local/gitversion-go ./cmd/gitversion-go
