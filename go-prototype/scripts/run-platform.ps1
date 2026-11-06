param(
    [Parameter(Mandatory = $true)][string]$Dotnet,
    [string]$Candidate = "$PSScriptRoot/../.local/platform-bin/windows-amd64/gitversion-go.exe",
    [string]$TestExecutable = "$PSScriptRoot/../.local/platform-bin/windows-amd64/parity.test.exe"
)
$ErrorActionPreference = 'Stop'
$oldDotnet = $env:PARITY_DOTNET
$oldCandidate = $env:PARITY_CANDIDATE
try {
    $env:PARITY_DOTNET = (Resolve-Path $Dotnet).Path
    $env:PARITY_CANDIDATE = (Resolve-Path $Candidate).Path
    $testPath = (Resolve-Path $TestExecutable).Path
    Push-Location "$PSScriptRoot/../cmd/parity"
    try {
        & $testPath '-test.run' '^TestPlatformCuratedParity$' '-test.v' '-test.timeout' '15m'
        if ($LASTEXITCODE -ne 0) { throw "Platform parity failed: exit $LASTEXITCODE" }
    } finally { Pop-Location }
} finally {
    $env:PARITY_DOTNET = $oldDotnet
    $env:PARITY_CANDIDATE = $oldCandidate
}
