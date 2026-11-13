namespace AlyPublish.Services;

/// <summary>A failure while reading or writing a project's .updator configuration.</summary>
public sealed class ProjectConfigException : Exception
{
    public ProjectConfigException(string message) : base(message)
    {
    }

    public ProjectConfigException(string message, Exception innerException) : base(message, innerException)
    {
    }
}
