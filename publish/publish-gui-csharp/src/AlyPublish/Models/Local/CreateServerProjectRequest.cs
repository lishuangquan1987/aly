namespace AlyPublish.Models.Local;

/// <summary>Context the add-project wizard passes to the create-server-project dialog.</summary>
public sealed record CreateServerProjectRequest(
    string ServerUrl,
    IReadOnlyList<string> ExistingProjectNames,
    IReadOnlyList<string> SuggestedIgnoreFolders,
    IReadOnlyList<string> SuggestedIgnoreFiles);
