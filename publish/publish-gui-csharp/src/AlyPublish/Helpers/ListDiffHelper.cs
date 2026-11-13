namespace AlyPublish.Helpers;

/// <summary>Computes the items to add and remove when editing a configuration list.</summary>
public static class ListDiffHelper
{
    public static ListDiff Compute(IReadOnlyList<string> original, IReadOnlyList<string> current)
    {
        var originalSet = new HashSet<string>(original, StringComparer.Ordinal);
        var currentSet = new HashSet<string>(current, StringComparer.Ordinal);

        var added = current
            .Where(item => !originalSet.Contains(item))
            .Distinct(StringComparer.Ordinal)
            .ToList();
        var removed = original
            .Where(item => !currentSet.Contains(item))
            .Distinct(StringComparer.Ordinal)
            .ToList();

        return new ListDiff(added, removed);
    }
}
