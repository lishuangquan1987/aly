namespace AlyPublish.Models.Local;

/// <summary>Input collected by the initialization wizard before running the CLI.</summary>
public sealed record ProjectInitializationRequest(
    string ServerUrl,
    string ProjectName,
    string ProjectPath,
    string DisplayName,
    string MainExePath,
    string CloseProcessName,
    IReadOnlyList<string> IgnoreFolders,
    IReadOnlyList<string> IgnoreFiles,
    IReadOnlyList<string> UnCopyFolders,
    IReadOnlyList<string> UnCopyFiles);
