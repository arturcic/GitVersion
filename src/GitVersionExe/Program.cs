using System;
using System.Reflection;
using System.Threading.Tasks;
using GitVersion.Extensions;
using GitVersion.MSBuildTask.LibGit2Sharp;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Options;

namespace GitVersion
{
    internal class Program
    {
        private readonly Action<IServiceCollection> overrides;

        internal Program(Action<IServiceCollection> overrides = null)
        {
            this.overrides = overrides;
        }

        private static async Task MainInner(string[] args)
        {
            await new Program().RunAsync(args);
        }

        public static async Task Main(string[] args)
        {
            LibGit2SharpLoader.LoadAssembly();

            var inContextAssembly = LibGit2SharpLoader.Instance.Assembly;
            var innerProgramType = inContextAssembly.GetType(typeof(Program).FullName);

            var mainInnerMethod = innerProgramType?.GetMethod(nameof(MainInner), BindingFlags.Static | BindingFlags.NonPublic);
            await (Task) mainInnerMethod?.Invoke(null, new object[] { args });
        }

        internal Task RunAsync(string[] args)
        {
            return CreateHostBuilder(args).Build().RunAsync();
        }

        private IHostBuilder CreateHostBuilder(string[] args) =>
            new HostBuilder()
                .ConfigureAppConfiguration((hostContext, configApp) =>
                {
                    configApp.AddCommandLine(args);
                })
                .ConfigureServices((hostContext, services) =>
                {
                    services.AddModule(new GitVersionCoreModule());
                    services.AddModule(new GitVersionExeModule());

                    services.AddSingleton(sp =>
                    {
                        var arguments = sp.GetService<IArgumentParser>().ParseArguments(args);
                        var gitVersionOptions = arguments.ToOptions();
                        return Options.Create(gitVersionOptions);
                    });

                    overrides?.Invoke(services);
                    services.AddHostedService<GitVersionApp>();
                })
                .UseConsoleLifetime();
    }
}
