using System.Collections.ObjectModel;
using System.Reflection;
using YamlDotNet.Core;
using YamlDotNet.Core.Events;
using YamlDotNet.Helpers;
using YamlDotNet.Serialization;
using YamlDotNet.Serialization.NamingConventions;
using YamlDotNet.Serialization.TypeInspectors;

namespace GitVersion.Configuration;

internal class ConfigurationSerializer : IConfigurationSerializer
{
    private static readonly ConfigurationStaticContext context = new();

    // Prevent reflection-based builders from installing DefaultFsharpHelper (via Build()),
    // which breaks nullable value type handling in the static serializer's graph traversal.
    static ConfigurationSerializer() => FsharpHelper.Instance = new NullFsharpHelper();

    private static IDeserializer Deserializer => new StaticDeserializerBuilder(context)
        .WithNamingConvention(HyphenatedNamingConvention.Instance)
        .WithTypeConverter(VersionStrategiesConverter.Instance)
        .WithTypeConverter(new HashSetStringConverter())
        .WithTypeConverter(new CollectionStringConverter())
        .WithTypeConverter(new StringStringDictionaryConverter())
        .WithTypeInspector(inspector => new JsonPropertyNameInspector(inspector), loc => loc.After<NamingConventionTypeInspector>())
        .Build();

    private static ISerializer Serializer => new StaticSerializerBuilder(context)
        .ConfigureDefaultValuesHandling(DefaultValuesHandling.OmitNull)
        .WithTypeConverter(VersionStrategiesConverter.Instance)
        .WithTypeConverter(new HashSetStringConverter())
        .WithTypeConverter(new CollectionStringConverter())
        .WithTypeConverter(new StringStringDictionaryConverter())
        .WithTypeInspector(inspector => new JsonPropertyNameInspector(inspector), loc => loc.After<NamingConventionTypeInspector>())
        .WithNamingConvention(HyphenatedNamingConvention.Instance).Build();

    // Reflection-based (de)serializers for untyped Dictionary<object, object?> operations
    // used by ConfigurationHelper/ConfigurationProvider for configuration merging.
#pragma warning disable IL2026, IL3050
    private static IDeserializer ReflectionDeserializer => new DeserializerBuilder()
        .WithNamingConvention(HyphenatedNamingConvention.Instance)
        .Build();

    private static ISerializer ReflectionSerializer => new SerializerBuilder()
        .ConfigureDefaultValuesHandling(DefaultValuesHandling.OmitNull)
        .WithNamingConvention(HyphenatedNamingConvention.Instance).Build();
#pragma warning restore IL2026, IL3050

    public T Deserialize<T>(string input) =>
        typeof(T) == typeof(Dictionary<object, object?>) ? ReflectionDeserializer.Deserialize<T>(input) : Deserializer.Deserialize<T>(input);

    public string Serialize(object graph) =>
        graph is IDictionary<object, object?> ? ReflectionSerializer.Serialize(graph) : Serializer.Serialize(graph);

    public IGitVersionConfiguration? ReadConfiguration(string input) => Deserializer.Deserialize<GitVersionConfiguration?>(input);

    private sealed class JsonPropertyNameInspector(ITypeInspector innerTypeDescriptor) : TypeInspectorSkeleton
    {
        public override string GetEnumName(Type enumType, string name) => innerTypeDescriptor.GetEnumName(enumType, name);

        public override string GetEnumValue(object enumValue) => innerTypeDescriptor.GetEnumValue(enumValue);

        public override IEnumerable<IPropertyDescriptor> GetProperties(Type type, object? container)
        {
            // Build reflection-based lookup for JsonPropertyName and JsonIgnore
            var reflectionProps = type.GetProperties(BindingFlags.Public | BindingFlags.Instance);
            var jsonNameMap = new Dictionary<string, string>();
            var jsonIgnoreSet = new HashSet<string>();
            foreach (var pi in reflectionProps)
            {
                var jpn = pi.GetCustomAttribute<JsonPropertyNameAttribute>();
                if (jpn is not null)
                    jsonNameMap[pi.Name] = jpn.Name;
                if (pi.GetCustomAttribute<JsonIgnoreAttribute>() is not null)
                    jsonIgnoreSet.Add(pi.Name);
            }

            return innerTypeDescriptor.GetProperties(type, container)
                .Where(p => !jsonIgnoreSet.Contains(p.Name))
                .Select(IPropertyDescriptor (p) =>
                {
                    if (jsonNameMap.TryGetValue(p.Name, out var alias))
                    {
                        var descriptor = new PropertyDescriptor(p);
                        descriptor.Name = alias;
                        return descriptor;
                    }
                    return p;
                })
                .OrderBy(p => p.Order);
        }
    }

    private sealed class HashSetStringConverter : IYamlTypeConverter
    {
        public bool Accepts(Type type) => type == typeof(HashSet<string>);

        public object ReadYaml(IParser parser, Type type, ObjectDeserializer rootDeserializer)
        {
            var set = new HashSet<string>();
            parser.Consume<SequenceStart>();
            while (!parser.TryConsume<SequenceEnd>(out _))
            {
                var scalar = parser.Consume<Scalar>();
                set.Add(scalar.Value);
            }
            return set;
        }

        public void WriteYaml(IEmitter emitter, object? value, Type type, ObjectSerializer serializer)
        {
            var set = (HashSet<string>?)value;
            emitter.Emit(new SequenceStart(null, null, false, SequenceStyle.Block));
            if (set is not null)
            {
                foreach (var item in set)
                    emitter.Emit(new Scalar(item));
            }
            emitter.Emit(new SequenceEnd());
        }
    }

    private sealed class CollectionStringConverter : IYamlTypeConverter
    {
        public bool Accepts(Type type) => type == typeof(Collection<string>);

        public object ReadYaml(IParser parser, Type type, ObjectDeserializer rootDeserializer)
        {
            var collection = new Collection<string>();
            parser.Consume<SequenceStart>();
            while (!parser.TryConsume<SequenceEnd>(out _))
            {
                var scalar = parser.Consume<Scalar>();
                collection.Add(scalar.Value);
            }
            return collection;
        }

        public void WriteYaml(IEmitter emitter, object? value, Type type, ObjectSerializer serializer)
        {
            var collection = (Collection<string>?)value;
            emitter.Emit(new SequenceStart(null, null, false, SequenceStyle.Block));
            if (collection is not null)
            {
                foreach (var item in collection)
                    emitter.Emit(new Scalar(item));
            }
            emitter.Emit(new SequenceEnd());
        }
    }

    private sealed class StringStringDictionaryConverter : IYamlTypeConverter
    {
        public bool Accepts(Type type) => type == typeof(Dictionary<string, string>);

        public object ReadYaml(IParser parser, Type type, ObjectDeserializer rootDeserializer)
        {
            var dict = new Dictionary<string, string>();
            parser.Consume<MappingStart>();
            while (!parser.TryConsume<MappingEnd>(out _))
            {
                var key = parser.Consume<Scalar>();
                var val = parser.Consume<Scalar>();
                dict[key.Value] = val.Value;
            }
            return dict;
        }

        public void WriteYaml(IEmitter emitter, object? value, Type type, ObjectSerializer serializer)
        {
            var dict = (Dictionary<string, string>?)value;
            emitter.Emit(new MappingStart(null, null, false, MappingStyle.Block));
            if (dict is not null)
            {
                foreach (var (key, val) in dict)
                {
                    emitter.Emit(new Scalar(key));
                    emitter.Emit(new Scalar(val));
                }
            }
            emitter.Emit(new MappingEnd());
        }
    }
}
