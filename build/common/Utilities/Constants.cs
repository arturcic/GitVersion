namespace Common.Utilities
{
    public class Constants
    {
        public const string RepoOwner = "GitTools";
        public const string Repository = "GitVersion";

        public const string Version50 = "5.0";
        public const string Version31 = "3.1";

        public const string NetVersion50 = "net5.0";
        public const string CoreFxVersion31 = "netcoreapp3.1";
        public const string FullFxVersion48 = "net48";
        public static readonly string[] VersionsToBuild = { Version50, Version31 };

        public const string DockerBaseImageName = "gittools/build-images";
        public const string DockerImageName = "gittools/gitversion";

        public const string DockerHub = "dockerhub";
        public const string GitHub = "github";
        public const string DockerHubRegistry = "docker.io";
        public const string GitHubContainerRegistry = "ghcr.io";

        public const string Alpine313 = "alpine.3.13";
        public const string Alpine314 = "alpine.3.14";
        public const string Debian10 = "debian.10";
        public const string Debian11 = "debian.11";
        public const string Ubuntu2004 = "ubuntu.20.04";
        public const string DockerDistroLatest = Debian10;
        public static readonly string[] DockerDistrosToBuild =
        {
            Alpine313,
            Alpine314,
            "centos.7",
            "centos.8",
            "debian.9",
            Debian10,
            Debian11,
            "fedora.33",
            "ubuntu.16.04",
            "ubuntu.18.04",
            Ubuntu2004
        };
        public const string NugetOrgUrl = "https://api.nuget.org/v3/index.json";
        public const string GithubPackagesUrl = "https://nuget.pkg.github.com/gittools/index.json";
        public const string ChocolateyUrl = "https://push.chocolatey.org/";
    }
}
