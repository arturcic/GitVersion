using Microsoft.Extensions.Logging;

namespace GitVersion.Extensions;

public static class LoggerExtensions
{
    /// <summary>
    /// Creates a logging scope that tracks the duration of an operation.
    /// Logs the start and end of the operation, including the elapsed time.
    /// </summary>
    /// <param name="logger">The logger instance</param>
    /// <param name="operationDescription">Description of the operation being performed</param>
    /// <returns>An IDisposable that when disposed, logs the completion and duration of the operation</returns>
    public static IDisposable BeginTimedOperation(this ILogger logger, string operationDescription)
    {
        ArgumentNullException.ThrowIfNull(logger);
        ArgumentNullException.ThrowIfNull(operationDescription);

        var stopwatch = Stopwatch.StartNew();
        logger.LogInformation("-< Begin: {OperationDescription} >-", operationDescription);

        return new TimedOperationScope(logger, operationDescription, stopwatch);
    }

    private sealed class TimedOperationScope : IDisposable
    {
        private readonly ILogger logger;
        private readonly string operationDescription;
        private readonly Stopwatch stopwatch;
        private bool disposed;

        public TimedOperationScope(ILogger logger, string operationDescription, Stopwatch stopwatch)
        {
            this.logger = logger;
            this.operationDescription = operationDescription;
            this.stopwatch = stopwatch;
        }

        public void Dispose()
        {
            if (disposed) return;
            disposed = true;

            stopwatch.Stop();
            logger.LogInformation("-< End: {OperationDescription} (Took: {DurationMs:N}ms) >-",
                operationDescription, stopwatch.Elapsed.TotalMilliseconds);
        }
    }
}
