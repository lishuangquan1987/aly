using System.Globalization;
using System.Text.RegularExpressions;

namespace AlyPublish.Helpers;

/// <summary>Suggests the next release version based on the last published one.</summary>
public static partial class VersionHelper
{
    [GeneratedRegex(@"^(?<prefix>[vV])?(?<numbers>\d+(?:\.\d+)*)$")]
    private static partial Regex VersionPattern();

    /// <returns>The incremented version, or <see langword="null"/> when the input is not a version.</returns>
    public static string? SuggestNext(string? currentVersion)
    {
        if (string.IsNullOrWhiteSpace(currentVersion))
        {
            return null;
        }

        var match = VersionPattern().Match(currentVersion.Trim());
        if (!match.Success)
        {
            return null;
        }

        var numbers = match.Groups["numbers"].Value.Split('.');
        if (!int.TryParse(numbers[^1], NumberStyles.None, CultureInfo.InvariantCulture, out var last)
            || last == int.MaxValue)
        {
            return null;
        }

        numbers[^1] = (last + 1).ToString(CultureInfo.InvariantCulture);
        return match.Groups["prefix"].Value + string.Join('.', numbers);
    }
}
