namespace GitVersion.BuildAgents.Tests;

[TestFixture]
public class GitHubReportingProof
{
    [Test]
    public void DeliberateFailureForHostedReportingProof()
    {
        if (System.Environment.GetEnvironmentVariable("MTP_REPORTING_PROOF") != "true")
        {
            Assert.Ignore("Temporary proof runs only in the MTP reporting pilot.");
        }

        Assert.Fail("Intentional failure for GitVersion #5220: verify the source annotation, failure summary and downloadable reports. This temporary test will be removed after verification.");
    }
}
