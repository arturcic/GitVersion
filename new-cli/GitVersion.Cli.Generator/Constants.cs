namespace GitVersion;

public static class Constants
{
    internal const string GeneratedNamespaceName = "GitVersion.Generated";
    internal const string CommonNamespaceName = "GitVersion";
    internal const string InfrastructureNamespaceName = "GitVersion.Infrastructure";
    internal const string CommandNamespaceName = "GitVersion.Commands";
    internal const string ExtensionsNamespaceName = "GitVersion.Extensions";

    /*language=cs*/
    internal const string GeneratedHeader =
"""
//------------------------------------------------------------------------------
// <auto-generated>
//     This code was generated by a tool.
//
//     Changes to this file may cause incorrect behavior and will be lost if
//     the code is regenerated.
// </auto-generated>
//------------------------------------------------------------------------------
#nullable enable
""";
}
