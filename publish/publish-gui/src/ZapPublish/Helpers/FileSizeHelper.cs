namespace ZapPublish.Helpers;

public static class FileSizeHelper
{
    private static readonly string[] Suffixes = { "B", "KB", "MB", "GB", "TB" };

    public static string FormatSize(long bytes)
    {
        if (bytes == 0) return "0 B";

        int order = 0;
        double size = bytes;
        while (size >= 1024 && order < Suffixes.Length - 1)
        {
            order++;
            size /= 1024;
        }
        return $"{size:0.##} {Suffixes[order]}";
    }

    public static string FormatSize(double bytes)
    {
        return FormatSize((long)bytes);
    }

    public static string GetSizeComparison(long localSize, long serverSize)
    {
        if (serverSize == 0)
            return FormatSize(localSize);
        
        if (localSize == serverSize)
            return $"{FormatSize(localSize)} (相同)";
        
        var diff = localSize - serverSize;
        var prefix = diff > 0 ? "+" : "";
        return $"{FormatSize(localSize)} ({prefix}{FormatSize(Math.Abs(diff))})";
    }
}
