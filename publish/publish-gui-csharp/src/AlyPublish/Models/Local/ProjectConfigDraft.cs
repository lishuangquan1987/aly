namespace AlyPublish.Models.Local;

/// <summary>The editable parts of .updator/shared.json, as edited by the settings dialog.</summary>
public sealed record ProjectConfigDraft(
    string ServerUrl,
    string ProjectName,
    IReadOnlyList<string> IgnoreFolders,
    IReadOnlyList<string> IgnoreFiles,
    IReadOnlyList<string> UnCopyFolders,
    IReadOnlyList<string> UnCopyFiles);
