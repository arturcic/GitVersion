using GitVersion.Extensions;
using GitVersion.OutputVariables;

namespace GitVersion.Agents;

internal class AppVeyor : ICurrentBuildAgent
{
    private readonly IEnvironment environment;

    public AppVeyor(IEnvironment environment) => this.environment = environment.NotNull();

    public const string EnvironmentVariableName = "APPVEYOR";

    public string EnvironmentVariable => EnvironmentVariableName;

    public bool CanApplyToCurrentContext() => !this.environment.GetEnvironmentVariable(EnvironmentVariable).IsNullOrEmpty();

    public string GenerateSetVersionMessage(GitVersionVariables variables)
    {
        var buildNumber = this.environment.GetEnvironmentVariable("APPVEYOR_BUILD_NUMBER");
        var apiUrl = this.environment.GetEnvironmentVariable("APPVEYOR_API_URL") ?? throw new Exception("APPVEYOR_API_URL environment variable not set");

        using var httpClient = GetHttpClient(apiUrl);

        var body = new
        {
            version = $"{variables.FullSemVer}.build.{buildNumber}"
        };

        var stringContent = new StringContent(JsonSerializer.Serialize(body), Encoding.UTF8, "application/json");

        try
        {
            var response = httpClient.PutAsync("api/build", stringContent).GetAwaiter().GetResult();
            response.EnsureSuccessStatusCode();
        }
        catch (Exception ex)
        {
            return $"Failed to set AppVeyor build number to '{variables.FullSemVer}'. The error was: {ex.Message}";
        }

        return $"Set AppVeyor build number to '{variables.FullSemVer}'.";
    }

    public string[] GenerateSetParameterMessage(string name, string? value)
    {
        var apiUrl = this.environment.GetEnvironmentVariable("APPVEYOR_API_URL") ?? throw new Exception("APPVEYOR_API_URL environment variable not set");
        var httpClient = GetHttpClient(apiUrl);

        var body = new
        {
            name = $"GitVersion_{name}",
            value = $"{value}"
        };

        var stringContent = new StringContent(JsonSerializer.Serialize(body), Encoding.UTF8, "application/json");
        var response = httpClient.PostAsync("api/build/variables", stringContent).GetAwaiter().GetResult();
        response.EnsureSuccessStatusCode();

        return new[]
        {
            $"Adding Environment Variable. name='GitVersion_{name}' value='{value}']"
        };
    }

    private static HttpClient GetHttpClient(string apiUrl) => new()
    {
        BaseAddress = new Uri(apiUrl)
    };

    public string? GetCurrentBranch(bool usingDynamicRepos)
    {
        var pullRequestBranchName = this.environment.GetEnvironmentVariable("APPVEYOR_PULL_REQUEST_HEAD_REPO_BRANCH");
        if (!pullRequestBranchName.IsNullOrWhiteSpace())
        {
            return pullRequestBranchName;
        }
        return this.environment.GetEnvironmentVariable("APPVEYOR_REPO_BRANCH");
    }

    public bool PreventFetch() => false;
}
