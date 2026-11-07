#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
root="$PWD"
dotnet_host="${DOTNET_HOST:-dotnet}"
"$dotnet_host" build oracle/RepositoryContracts/RepositoryContracts.csproj \
  -c Release -o .local/repository-contract-oracle --nologo \
  "-p:BaselineDirectory=$root/.local/baseline" \
  "-p:BaseIntermediateOutputPath=$root/.local/repository-contract-oracle-obj/"
