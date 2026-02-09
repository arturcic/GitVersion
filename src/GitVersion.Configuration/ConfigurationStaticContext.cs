using GitVersion.VersionCalculation;
using YamlDotNet.Serialization;

namespace GitVersion.Configuration;

[YamlStaticContext]
[YamlSerializable(typeof(GitVersionConfiguration))]
[YamlSerializable(typeof(BranchConfiguration))]
[YamlSerializable(typeof(IgnoreConfiguration))]
[YamlSerializable(typeof(PreventIncrementConfiguration))]
[YamlSerializable(typeof(AssemblyVersioningScheme))]
[YamlSerializable(typeof(AssemblyFileVersioningScheme))]
[YamlSerializable(typeof(SemanticVersionFormat))]
[YamlSerializable(typeof(DeploymentMode))]
[YamlSerializable(typeof(IncrementStrategy))]
[YamlSerializable(typeof(CommitMessageIncrementMode))]
[YamlSerializable(typeof(VersionStrategies))]
public partial class ConfigurationStaticContext : StaticContext;
