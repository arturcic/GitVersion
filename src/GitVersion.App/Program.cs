using GitVersion.Agents;
using GitVersion.Configuration;
using GitVersion.Extensions;
using GitVersion.Output;
using Microsoft.Extensions.DependencyInjection;
using Spectre.Console.Cli;
using GitVersion.Commands;
using GitVersion.Infrastructure;

namespace GitVersion;

internal class Program
{
    private readonly Action<IServiceCollection>? overrides;

    internal Program(Action<IServiceCollection>? overrides = null) => this.overrides = overrides;

    private static async Task<int> Main(string[] args) => await new Program().RunAsync(args);

    internal async Task<int> RunAsync(string[] args)
    {
        var services = new ServiceCollection();
        ConfigureServices(services); // Populate our services
        this.overrides?.Invoke(services);

        var registrar = new TypeRegistrar(services);
        var app = new CommandApp<DefaultCommand>(registrar);

        app.Configure(config =>
        {
#if DEBUG
            config.ValidateExamples();
            config.PropagateExceptions();
#endif
            // Other configurations can go here (e.g., exception handler)
        });

        return await app.RunAsync(args);
    }

    private void ConfigureServices(IServiceCollection services)
    {
        // Register all services needed by GitVersion and its commands
        services.AddModule(new GitVersionCoreModule());
        services.AddModule(new GitVersionLibGit2SharpModule());
        services.AddModule(new GitVersionBuildAgentsModule());
        services.AddModule(new GitVersionConfigurationModule());
        services.AddModule(new GitVersionOutputModule());
        services.AddModule(new GitVersionAppModule()); // This registers IGitVersionExecutor, IHelpWriter etc.

        // DefaultCommand is automatically registered by CommandApp<TCommand>(registrar)
        // IGitVersionExecutor (dependency of DefaultCommand) is registered via GitVersionAppModule.
        // Other services required by IGitVersionExecutor or other parts of GitVersionCoreModule etc.
        // are expected to be registered within their respective modules.

        // We don't need to parse args for GitVersionOptions here anymore,
        // as DefaultCommand will receive GitVersionSettings and map it.
    }
}
