namespace Common.Utilities;

public static class BuildRunner
{
    public static int Run(ICakeContext context, Action runTarget)
    {
        try
        {
            runTarget();
            return 0;
        }
        catch (Exception exception)
        {
            context.Error("{0}", exception);
            // Preserve the stage command's existing failure code (-1; 255 on Unix).
            return -1;
        }
    }
}
