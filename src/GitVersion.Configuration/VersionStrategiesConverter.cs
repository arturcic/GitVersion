using GitVersion.VersionCalculation;
using YamlDotNet.Core;
using YamlDotNet.Core.Events;
using YamlDotNet.Serialization;

namespace GitVersion.Configuration;

internal class VersionStrategiesConverter : IYamlTypeConverter
{
    public static readonly IYamlTypeConverter Instance = new VersionStrategiesConverter();

    public bool Accepts(Type type) => type == typeof(VersionStrategies[]);

    public object ReadYaml(IParser parser, Type type, ObjectDeserializer rootDeserializer)
    {
        List<VersionStrategies> strategies = [];

        if (parser.TryConsume<SequenceStart>(out _))
        {
            while (!parser.TryConsume<SequenceEnd>(out _))
            {
                var data = parser.Consume<Scalar>().Value;
                strategies.Add(Enum.Parse<VersionStrategies>(data));
            }
        }
        else
        {
            // Handle legacy JSON array format: ["Fallback", "ConfiguredNextVersion", ...]
            var data = parser.Consume<Scalar>().Value.Trim();
            if (data.StartsWith('[') && data.EndsWith(']'))
            {
                foreach (var item in data[1..^1].Split(','))
                {
                    var val = item.Trim().Trim('"');
                    if (!string.IsNullOrWhiteSpace(val))
                        strategies.Add(Enum.Parse<VersionStrategies>(val));
                }
            }
            else
            {
                strategies.Add(Enum.Parse<VersionStrategies>(data));
            }
        }

        return strategies.ToArray();
    }

    public void WriteYaml(IEmitter emitter, object? value, Type type, ObjectSerializer serializer)
    {
        var strategies = (VersionStrategies[])value!;

        emitter.Emit(new SequenceStart(null, null, false, SequenceStyle.Block));
        foreach (var strategy in strategies)
            emitter.Emit(new Scalar(strategy.ToString()));
        emitter.Emit(new SequenceEnd());
    }
}
