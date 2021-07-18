param(
    [parameter(Mandatory=$true, Position=0)][string] $runtime,
    [parameter(Mandatory=$true, Position=1)][string] $repoPath
)

if (Test-Path /native/linux-x64/gitversion -PathType Leaf) {
    Write-Host "it exists"
} else {
    { Write-Host "it does not exist" }
}
& "/native/linux-x64/gitversion" /repo /showvariable FullSemver;
