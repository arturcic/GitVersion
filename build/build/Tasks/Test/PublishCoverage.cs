using Common.Utilities;

namespace Build.Tasks;

public class PublishCoverage
{
    public bool ShouldRun(BuildContext context)
    {
        var shouldRun = true;
        shouldRun &= context.ShouldRun(context.IsOnWindows, $"{nameof(PublishCoverage)} works only on Windows agents.");
        shouldRun &= context.ShouldRun(context.IsReleaseBranchOriginalRepo, $"{nameof(PublishCoverage)} works only for main or release branch, original repository.");

        return shouldRun;
    }

    public void Run(BuildContext context)
    {
        var coverageFiles = context
            .GetFiles($"{Paths.Src}/**/coverage.*.xml")
            .Select(file => context.MakeRelative(file).ToString()).ToArray();

        var token = context.Credentials?.CodeCov?.Token;
        if (string.IsNullOrEmpty(token))
        {
            throw new InvalidOperationException("Could not resolve CodeCov token.");
        }

        if (coverageFiles.Length == 0)
        {
            context.Warning("No coverage files found.");
            return;
        }

        context.Codecov(new CodecovSettings
        {
            Files = coverageFiles,
            Token = token
        });
    }
}
