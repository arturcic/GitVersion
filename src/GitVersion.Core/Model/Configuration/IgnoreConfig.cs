using System.Text.Json.Serialization;
using GitVersion.Attributes;
using GitVersion.VersionCalculation;

namespace GitVersion.Model.Configuration;

public class IgnoreConfig
{
    public IgnoreConfig() => ShAs = Enumerable.Empty<string>();

    [JsonPropertyName("commits-before")]
    [JsonPropertyDescription("Commits before this date will be ignored. Format: yyyy-MM-ddTHH:mm:ss.")]
    [JsonPropertyPattern("'yyyy-MM-ddTHH:mm:ss'", PatternFormat.DateTime)]
    public DateTimeOffset? Before { get; set; }

    [JsonPropertyName("sha")]
    [JsonPropertyDescription("A sequence of SHAs to be excluded from the version calculations.")]
    public IEnumerable<string> ShAs { get; set; }

    [JsonIgnore]
    public virtual bool IsEmpty => Before == null && ShAs.Any() == false;

    public virtual IEnumerable<IVersionFilter> ToFilters()
    {
        if (ShAs.Any()) yield return new ShaVersionFilter(ShAs);
        if (Before.HasValue) yield return new MinDateVersionFilter(Before.Value);
    }
}
