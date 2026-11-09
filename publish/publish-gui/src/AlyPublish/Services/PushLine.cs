using AlyPublish.Models.Cli;

namespace AlyPublish.Services;

public enum PushLineKind
{
    Ignored,
    Progress,
    Success,
    Failure
}

/// <summary>One JSON line emitted by <c>aly-publish push --json</c>.</summary>
public readonly record struct PushLine(PushLineKind Kind, UploadProgress? Progress, string? ErrorMessage);
