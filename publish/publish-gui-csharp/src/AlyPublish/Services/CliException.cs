namespace AlyPublish.Services;

/// <summary>A failure reported by the aly-publish CLI, with a user-readable message.</summary>
public sealed class CliException : Exception
{
    public CliException(string message) : base(message)
    {
    }

    public CliException(string message, Exception innerException) : base(message, innerException)
    {
    }
}
