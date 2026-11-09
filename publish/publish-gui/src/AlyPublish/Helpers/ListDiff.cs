namespace AlyPublish.Helpers;

/// <summary>The difference between two configuration lists.</summary>
public readonly record struct ListDiff(IReadOnlyList<string> Added, IReadOnlyList<string> Removed);
