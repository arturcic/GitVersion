#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
root="$PWD"
dotnet_host="${DOTNET_HOST:-dotnet}"
"$dotnet_host" build oracle/DualBackend/DualBackend.csproj \
  -c Release -o .local/dual-backend-oracle --nologo \
  "-p:BaselineDirectory=$root/.local/baseline" \
  "-p:BaseIntermediateOutputPath=$root/.local/dual-backend-oracle-obj/"
if [[ ${OS:-} != Windows_NT ]]; then
  for native in libgit2-5853918.dylib libgit2-5853918.so; do
    cp ".local/repository-contract-oracle/$native" ".local/dual-backend-oracle/$native"
  done
fi
