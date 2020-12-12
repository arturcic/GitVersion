using System;
using System.IO;
using System.Reflection;

namespace GitVersion.MSBuildTask.LibGit2Sharp
{
    public class LibGit2SharpLoader
    {
        public static LibGit2SharpLoader Instance { get; private set; }
        public Assembly Assembly { get; }

        public static void LoadAssembly() => Instance = new LibGit2SharpLoader();

#if NETFRAMEWORK
        private LibGit2SharpLoader()
        {
            AppDomain.CurrentDomain.AssemblyResolve += AssemblyResolve;
            var assemblyName = typeof(LibGit2SharpLoader).Assembly.GetName();
            Assembly = Assembly.Load(assemblyName);
        }
        
        private static Assembly AssemblyResolve(object sender, ResolveEventArgs args)
        {
            var assemblyDirectory = Path.GetDirectoryName(typeof(LibGit2SharpLoader).Assembly.Location);
            var referenceName = new AssemblyName(args.Name);
            if (!referenceName.Name.StartsWith("System.", StringComparison.OrdinalIgnoreCase))
                return null;

            if (referenceName.Version != new Version(0, 0, 0, 0))
                return null;

            var referencePath = Path.Combine(assemblyDirectory, referenceName.Name + ".dll");
            return !File.Exists(referencePath) ? null : Assembly.Load(AssemblyName.GetAssemblyName(referencePath));
        }
#endif

#if !NETFRAMEWORK
        private LibGit2SharpLoader()
        {
            var type = typeof(LibGit2SharpLoader);
            var entryAssembly = new Uri(type.GetTypeInfo().Assembly.CodeBase).LocalPath;
            Assembly = GitLoaderContext.Instance.LoadFromAssemblyPath(entryAssembly);
        }

        private class GitLoaderContext : System.Runtime.Loader.AssemblyLoadContext
        {
            public static readonly GitLoaderContext Instance = new GitLoaderContext();

            protected override Assembly Load(AssemblyName assemblyName)
            {
                var path = Path.Combine(Path.GetDirectoryName(typeof(GitLoaderContext).Assembly.Location), assemblyName.Name + ".dll");
                return File.Exists(path)
                    ? LoadFromAssemblyPath(path)
                    : Default.LoadFromAssemblyName(assemblyName);
            }
        }
#endif
    }
}
