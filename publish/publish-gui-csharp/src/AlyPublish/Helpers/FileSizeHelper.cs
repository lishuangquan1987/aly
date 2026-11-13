using System.Globalization;

namespace AlyPublish.Helpers;

/// <summary>Formats byte counts for display in the file and upload tables.</summary>
public static class FileSizeHelper
{
    private static readonly string[] Units = ["B", "KB", "MB", "GB", "TB"];

    public static string Format(long bytes)
    {
        if (bytes <= 0)
        {
            return "0 B";
        }

        double size = bytes;
        var unitIndex = 0;
        while (size >= 1024 && unitIndex < Units.Length - 1)
        {
            size /= 1024;
            unitIndex++;
        }

        return string.Create(CultureInfo.InvariantCulture, $"{size:0.##} {Units[unitIndex]}");
    }
}
