namespace AlyPublish.Models.Local;

/// <summary>The initialized project plus any non-fatal warnings to surface to the user.</summary>
public sealed record ProjectSetupResult(ProjectConfig Config, IReadOnlyList<string> Warnings);
