using GitVersion.Configuration;
using GitVersion.Core.Tests.Helpers;
using GitVersion.Extensions;
using GitVersion.Helpers;
using GitVersion.Logging;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Options;

namespace GitVersion.Core.Tests;

[TestFixture]
public static class ConfigurationFileLocatorTests
{
    public class DefaultConfigFileLocatorTests : TestBase
    {
        private string repoPath;
        private string workingPath;
        private IFileSystem fileSystem;
        private ConfigurationProvider configurationProvider;
        private IConfigurationFileLocator configFileLocator;

        [SetUp]
        public void Setup()
        {
            this.repoPath = PathHelper.Combine(Path.GetTempPath(), "MyGitRepo");
            this.workingPath = PathHelper.Combine(this.repoPath, "Working");
            var options = Options.Create(new GitVersionOptions { WorkingDirectory = repoPath });

            var sp = ConfigureServices(services => services.AddSingleton(options));

            this.fileSystem = sp.GetRequiredService<IFileSystem>();
            this.configurationProvider = (ConfigurationProvider)sp.GetRequiredService<IConfigurationProvider>();
            this.configFileLocator = sp.GetRequiredService<IConfigurationFileLocator>();

            ShouldlyConfiguration.ShouldMatchApprovedDefaults.LocateTestMethodUsingAttribute<TestAttribute>();
        }

        [TestCase(ConfigurationFileLocator.DefaultFileName, ConfigurationFileLocator.DefaultFileName)]
        [TestCase(ConfigurationFileLocator.DefaultFileName, ConfigurationFileLocator.DefaultAlternativeFileName)]
        [TestCase(ConfigurationFileLocator.DefaultAlternativeFileName, ConfigurationFileLocator.DefaultFileName)]
        [TestCase(ConfigurationFileLocator.DefaultAlternativeFileName, ConfigurationFileLocator.DefaultAlternativeFileName)]
        public void ThrowsExceptionOnAmbiguousConfigFileLocation(string repoConfigFile, string workingConfigFile)
        {
            using var _ = SetupConfigFileContent(string.Empty, repoConfigFile, this.repoPath);
            using var __ = SetupConfigFileContent(string.Empty, workingConfigFile, this.workingPath);

            var repoConfigFilePath = PathHelper.Combine(this.repoPath, repoConfigFile);
            var workingDirectoryConfigFilePath = PathHelper.Combine(this.workingPath, workingConfigFile);
            var exception = Should.Throw<WarningException>(() => this.configFileLocator.Verify(this.workingPath, this.repoPath));

            var expectedMessage = $"Ambiguous configuration file selection from '{workingDirectoryConfigFilePath}' and '{repoConfigFilePath}'";
            exception.Message.ShouldBe(expectedMessage);
        }

        [TestCase(ConfigurationFileLocator.DefaultFileName)]
        [TestCase(ConfigurationFileLocator.DefaultAlternativeFileName)]
        public void NoWarnOnGitVersionYmlFile(string configurationFile)
        {
            using var _ = SetupConfigFileContent(string.Empty, configurationFile, this.repoPath);

            Should.NotThrow(() => this.configurationProvider.ProvideForDirectory(this.repoPath));
        }

        [Test]
        public void NoWarnOnNoGitVersionYmlFile() => Should.NotThrow(() => this.configurationProvider.ProvideForDirectory(this.repoPath));

        private IDisposable SetupConfigFileContent(string text, string fileName, string path)
        {
            var fullPath = PathHelper.Combine(path, fileName);
            var directory = this.fileSystem.Directory.CreateDirectory(Path.GetDirectoryName(fullPath)!);
            this.fileSystem.File.WriteAllText(fullPath, text);
            return Disposable.Create(() =>
            {
                this.fileSystem.File.Delete(fullPath);
                directory.Delete(true);
            });
        }
    }

    public class NamedConfigurationFileLocatorTests : TestBase
    {
        private const string myConfigYaml = "my-config.yaml";

        private string repoPath;
        private string workingPath;
        private IFileSystem fileSystem;
        private IConfigurationFileLocator configFileLocator;
        private GitVersionOptions gitVersionOptions;

        [SetUp]
        public void Setup()
        {
            this.gitVersionOptions = new GitVersionOptions { ConfigurationInfo = { ConfigurationFile = myConfigYaml } };
            this.repoPath = PathHelper.Combine(Path.GetTempPath(), "MyGitRepo");
            this.workingPath = PathHelper.Combine(this.repoPath, "Working");

            ShouldlyConfiguration.ShouldMatchApprovedDefaults.LocateTestMethodUsingAttribute<TestAttribute>();
        }

        [Test]
        public void ThrowsExceptionOnAmbiguousConfigFileLocation()
        {
            var sp = GetServiceProvider(this.gitVersionOptions);
            this.configFileLocator = sp.GetRequiredService<IConfigurationFileLocator>();
            this.fileSystem = sp.GetRequiredService<IFileSystem>();

            using var _ = SetupConfigFileContent(string.Empty, path: this.repoPath);
            using var __ = SetupConfigFileContent(string.Empty, path: this.workingPath);

            var repositoryConfigFilePath = PathHelper.Combine(this.repoPath, myConfigYaml);
            var workingDirectoryConfigFilePath = PathHelper.Combine(this.workingPath, myConfigYaml);

            var exception = Should.Throw<WarningException>(() => this.configFileLocator.Verify(this.workingPath, this.repoPath));

            var expectedMessage = $"Ambiguous configuration file selection from '{workingDirectoryConfigFilePath}' and '{repositoryConfigFilePath}'";
            exception.Message.ShouldBe(expectedMessage);
        }

        [Test]
        public void DoNotThrowWhenWorkingAndRepoPathsAreSame()
        {
            this.workingPath = this.repoPath;

            var sp = GetServiceProvider(this.gitVersionOptions);
            this.configFileLocator = sp.GetRequiredService<IConfigurationFileLocator>();
            this.fileSystem = sp.GetRequiredService<IFileSystem>();

            using var _ = SetupConfigFileContent(string.Empty, path: this.workingPath);

            Should.NotThrow(() => this.configFileLocator.Verify(this.workingPath, this.repoPath));
        }

        [Test]
        public void DoNotThrowWhenWorkingAndRepoPathsAreSame_WithDifferentCasing()
        {
            this.workingPath = this.repoPath.ToLower();

            var sp = GetServiceProvider(this.gitVersionOptions);
            this.configFileLocator = sp.GetRequiredService<IConfigurationFileLocator>();
            this.fileSystem = sp.GetRequiredService<IFileSystem>();

            using var _ = SetupConfigFileContent(string.Empty, path: this.workingPath);

            Should.NotThrow(() => this.configFileLocator.Verify(this.workingPath, this.repoPath));
        }

        [Test]
        public void DoNotThrowWhenConfigFileIsInSubDirectoryOfRepoPath()
        {
            this.workingPath = this.repoPath;

            this.gitVersionOptions = new GitVersionOptions { ConfigurationInfo = { ConfigurationFile = "./src/my-config.yaml" } };
            var sp = GetServiceProvider(this.gitVersionOptions);
            this.configFileLocator = sp.GetRequiredService<IConfigurationFileLocator>();
            this.fileSystem = sp.GetRequiredService<IFileSystem>();

            using var _ = SetupConfigFileContent(string.Empty, path: this.workingPath);

            Should.NotThrow(() => this.configFileLocator.Verify(this.workingPath, this.repoPath));
        }

        [Test]
        public void NoWarnOnCustomYmlFile()
        {
            var stringLogger = string.Empty;
            void Action(string info) => stringLogger = info;

            var logAppender = new TestLogAppender(Action);
            var log = new Log(logAppender);

            var sp = GetServiceProvider(this.gitVersionOptions, log);
            this.configFileLocator = sp.GetRequiredService<IConfigurationFileLocator>();
            this.fileSystem = sp.GetRequiredService<IFileSystem>();

            using var _ = SetupConfigFileContent(string.Empty);

            var configurationProvider = (ConfigurationProvider)sp.GetRequiredService<IConfigurationProvider>();

            configurationProvider.ProvideForDirectory(this.repoPath);
            stringLogger.Length.ShouldBe(0);
        }

        [Test]
        public void NoWarnOnCustomYmlFileOutsideRepoPath()
        {
            var stringLogger = string.Empty;
            void Action(string info) => stringLogger = info;

            var logAppender = new TestLogAppender(Action);
            var log = new Log(logAppender);

            var sp = GetServiceProvider(this.gitVersionOptions, log);
            this.configFileLocator = sp.GetRequiredService<IConfigurationFileLocator>();
            this.fileSystem = sp.GetRequiredService<IFileSystem>();

            using var _ = SetupConfigFileContent(string.Empty, path: PathHelper.GetRepositoryTempPath());

            var configurationProvider = (ConfigurationProvider)sp.GetRequiredService<IConfigurationProvider>();

            configurationProvider.ProvideForDirectory(this.repoPath);
            stringLogger.Length.ShouldBe(0);
        }

        [Test]
        public void ThrowsExceptionOnCustomYmlFileDoesNotExist()
        {
            var sp = GetServiceProvider(this.gitVersionOptions);
            this.configFileLocator = sp.GetRequiredService<IConfigurationFileLocator>();

            var exception = Should.Throw<WarningException>(() => this.configFileLocator.Verify(this.workingPath, this.repoPath));

            var configurationFile = this.gitVersionOptions.ConfigurationInfo.ConfigurationFile;
            var workingPathFileConfig = PathHelper.Combine(this.workingPath, configurationFile);
            var repoPathFileConfig = PathHelper.Combine(this.repoPath, configurationFile);
            var expectedMessage = $"The configuration file was not found at '{workingPathFileConfig}' or '{repoPathFileConfig}'";
            exception.Message.ShouldBe(expectedMessage);
        }

        private IDisposable SetupConfigFileContent(string text, string? fileName = null, string? path = null)
        {
            if (fileName.IsNullOrEmpty())
            {
                fileName = gitVersionOptions.ConfigurationInfo.ConfigurationFile;
            }
            var filePath = fileName;
            if (path.IsNullOrEmpty())
            {
                path = PathHelper.GetRepositoryTempPath();
            }
            filePath = PathHelper.Combine(path, filePath);
            var directory = this.fileSystem.Directory.CreateDirectory(Path.GetDirectoryName(filePath)!);

            this.fileSystem.File.WriteAllText(filePath, text);

            return Disposable.Create(() =>
            {
                this.fileSystem.File.Delete(filePath);
                directory.Delete(true);
            });
        }

        private static IServiceProvider GetServiceProvider(GitVersionOptions gitVersionOptions, ILog? log = null) =>
            ConfigureServices(services =>
            {
                if (log != null) services.AddSingleton(log);
                services.AddSingleton(Options.Create(gitVersionOptions)).AddSingleton<IFileSystem>(new FileSystem());
            });
    }
}
