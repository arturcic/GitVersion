param(
    [parameter(Mandatory=$true, Position=0)][string] $runtime,
    [parameter(Mandatory=$true, Position=1)][string] $repoPath
)

& "/native/linux-x64/gitversion" /repo /showvariable FullSemver;
