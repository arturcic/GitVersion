//go:build platform

package main

import "testing"

func TestPlatformProjectFileParity(t *testing.T) {
	const project = `<?xml version="1.0" encoding="utf-8"?>
<Project Sdk='Microsoft.NET.Sdk'>
    <!-- Project metadata -->
    <PropertyGroup><TargetFramework>net10.0</TargetFramework><AssemblyVersion>0.0.0.1</AssemblyVersion></PropertyGroup>
    <PropertyGroup Condition="'$(Configuration)' == 'Debug'">
        <AssemblyVersion>0.0.0.2</AssemblyVersion><AssemblyVersion>0.0.0.3</AssemblyVersion>
        <Nullable />
    </PropertyGroup>
</Project>
`
	const sql = `<Project Sdk="Microsoft.Build.Sql/2.0.0">
  <PropertyGroup><Name>Database &amp; schema</Name></PropertyGroup>
  <ItemGroup><Build Include="Tables/*.sql"/></ItemGroup>
</Project>`
	const nested = `<Project Sdk="Microsoft.NET.Sdk">
    <Choose><When Condition="true"><PropertyGroup><AssemblyVersion>0.0.0.0</AssemblyVersion></PropertyGroup></When></Choose>
    <PropertyGroup><GenerateAssemblyInfo>0</GenerateAssemblyInfo></PropertyGroup>
    <PropertyGroup><GenerateAssemblyInfo>1</GenerateAssemblyInfo><Note><![CDATA[<metadata>]]></Note></PropertyGroup>
</Project>`
	const custom = `<Project Sdk="Microsoft.NET.Sdk" xmlns:ext="urn:external">
    <ext:PropertyGroup><ext:AssemblyVersion>leave</ext:AssemblyVersion></ext:PropertyGroup>
    <PropertyGroup><ext:AssemblyVersion>leave</ext:AssemblyVersion><Description>one &lt; two &gt; zero</Description></PropertyGroup>
</Project>`
	testFileUpdaterParity(t, "project-file", "--update-project-files", []fileUpdaterCase{
		{name: "discover-csharp", path: "Test.csproj", original: project},
		{name: "discover-fsharp", path: "Test.fsproj", original: project},
		{name: "discover-visual-basic", path: "Test.vbproj", original: project},
		{name: "explicit-sql", path: "Test.sqlproj", original: sql, explicit: true},
		{name: "nested-property-group", path: "Test.csproj", original: nested, explicit: true},
		{name: "qualified-elements", path: "Test.csproj", original: custom, explicit: true},
		{name: "assembly-generation-disabled", path: "Test.csproj", original: `<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><GenerateAssemblyInfo>false</GenerateAssemblyInfo></PropertyGroup></Project>`, skip: true},
	})
}
