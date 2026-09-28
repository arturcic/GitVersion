namespace Common.Utilities;

public static class TaskHelp
{
    public static void Show(ICakeContext context, IEnumerable<ICakeTaskInfo> tasks)
    {
        var assembly = Assembly.GetEntryAssembly()!;
        context.Information($"Available targets:{Environment.NewLine}");
        foreach (var task in tasks.Where(task => !task.Name.Contains("Internal"))
                     .OrderBy(task => task.Name != "Default"))
        {
            var type = assembly.GetExportedTypes().FirstOrDefault(type => type.Name == task.Name);
            var arguments = type?.GetTaskArguments() ?? string.Empty;
            context.Information($"# {task.Description}");
            var target = task.Name != "Default" ? $"-Target {task.Name}" : string.Empty;
            context.Information($"  ./build.ps1 -Stage {assembly.GetName().Name} {target} {arguments}\n");
        }
    }
}
