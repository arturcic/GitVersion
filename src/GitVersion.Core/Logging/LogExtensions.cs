using GitVersion.Helpers;

namespace GitVersion.Logging;

public static class LogExtensions
{
    public static void Debug(this ILog log, string format, params object[] args) => log.Write(LogLevel.Debug, format, args);

    public static void Debug(this ILog log, Verbosity verbosity, string format, params object[] args) => log.Write(verbosity, LogLevel.Debug, format, args);

    public static void Debug(this ILog log, LogAction logAction) => log.Write(LogLevel.Debug, logAction);

    public static void Debug(this ILog log, Verbosity verbosity, LogAction logAction) => log.Write(verbosity, LogLevel.Debug, logAction);

    public static void Warning(this ILog log, string format, params object[] args) => log.Write(LogLevel.Warn, format, args);

    public static void Warning(this ILog log, Verbosity verbosity, string format, params object[] args) => log.Write(verbosity, LogLevel.Warn, format, args);

    public static void Warning(this ILog log, LogAction logAction) => log.Write(LogLevel.Warn, logAction);

    public static void Warning(this ILog log, Verbosity verbosity, LogAction logAction) => log.Write(verbosity, LogLevel.Warn, logAction);

    public static void Info(this ILog log, string format, params object[] args) => log.Write(LogLevel.Info, format, args);

    public static void Info(this ILog log, Verbosity verbosity, string format, params object[] args) => log.Write(verbosity, LogLevel.Info, format, args);

    public static void Info(this ILog log, LogAction logAction) => log.Write(LogLevel.Info, logAction);

    public static void Info(this ILog log, Verbosity verbosity, LogAction logAction) => log.Write(verbosity, LogLevel.Info, logAction);

    public static void Verbose(this ILog log, string format, params object[] args) => log.Write(LogLevel.Verbose, format, args);

    public static void Verbose(this ILog log, Verbosity verbosity, string format, params object[] args) => log.Write(verbosity, LogLevel.Verbose, format, args);

    public static void Verbose(this ILog log, LogAction logAction) => log.Write(LogLevel.Verbose, logAction);

    public static void Verbose(this ILog log, Verbosity verbosity, LogAction logAction) => log.Write(verbosity, LogLevel.Verbose, logAction);

    public static void Error(this ILog log, string format, params object[] args) => log.Write(LogLevel.Error, format, args);

    public static void Error(this ILog log, Verbosity verbosity, string format, params object[] args) => log.Write(verbosity, LogLevel.Error, format, args);

    public static void Error(this ILog log, LogAction logAction) => log.Write(LogLevel.Error, logAction);

    public static void Error(this ILog log, Verbosity verbosity, LogAction logAction) => log.Write(verbosity, LogLevel.Error, logAction);

    private static void Write(this ILog log, LogLevel level, string format, params object[] args)
    {
        var verbosity = GetVerbosityForLevel(level);
        if (verbosity > log.Verbosity)
        {
            return;
        }

        log.Write(verbosity, level, format, args);
    }

    private static void Write(this ILog log, Verbosity verbosity, LogLevel level, LogAction? logAction)
    {
        if (logAction == null)
            return;

        if (verbosity > log.Verbosity)
        {
            return;
        }

        logAction(ActionEntry);
        return;

        void ActionEntry(string format, object[] args) => log.Write(verbosity, level, format, args);
    }

    private static void Write(this ILog log, LogLevel level, LogAction? logAction)
    {
        if (logAction == null)
            return;

        var verbosity = GetVerbosityForLevel(level);
        if (verbosity > log.Verbosity)
        {
            return;
        }

        logAction(ActionEntry);
        return;

        void ActionEntry(string format, object[] args) => log.Write(verbosity, level, format, args);
    }

    public static IDisposable QuietVerbosity(this ILog log) => log.WithVerbosity(Verbosity.Quiet);

    public static IDisposable MinimalVerbosity(this ILog log) => log.WithVerbosity(Verbosity.Minimal);

    public static IDisposable NormalVerbosity(this ILog log) => log.WithVerbosity(Verbosity.Normal);

    public static IDisposable VerboseVerbosity(this ILog log) => log.WithVerbosity(Verbosity.Verbose);

    public static IDisposable DiagnosticVerbosity(this ILog log) => log.WithVerbosity(Verbosity.Diagnostic);

    private static IDisposable WithVerbosity(this ILog log, Verbosity verbosity)
    {
        ArgumentNullException.ThrowIfNull(log);
        var lastVerbosity = log.Verbosity;
        log.Verbosity = verbosity;
        return Disposable.Create(() => log.Verbosity = lastVerbosity);
    }

    private static Verbosity GetVerbosityForLevel(LogLevel level) => VerbosityMaps[level];

    private static readonly Dictionary<LogLevel, Verbosity> VerbosityMaps = new()
    {
        { LogLevel.Verbose, Verbosity.Verbose },
        { LogLevel.Debug, Verbosity.Diagnostic },
        { LogLevel.Info, Verbosity.Normal },
        { LogLevel.Warn, Verbosity.Minimal },
        { LogLevel.Error, Verbosity.Quiet },
        { LogLevel.Fatal, Verbosity.Quiet }
    };
}
