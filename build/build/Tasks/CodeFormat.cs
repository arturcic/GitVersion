using Common.Utilities;

namespace Build.Tasks;

public class CodeFormat
{
    public void Run(BuildContext context)
    {
        context.Information("Code format...");
        context.DotNetFormat(Paths.Build.FullPath);
        context.DotNetFormat(Paths.Src.FullPath, new DotNetFormatSettings
        {
            Exclude = [" **/AddFormats/"]
        });
    }
}
