using Avalonia.Data.Converters;
using System.Globalization;

namespace PublishTool.Converters;

public class BytesToSizeConverter : IValueConverter
{
    public object? Convert(object? value, Type targetType, object? parameter, CultureInfo culture)
    {
        long bytes = 0;
        if (value is long l)
            bytes = l;
        else if (value is int i)
            bytes = i;
        else if (value is double d)
            bytes = (long)d;
        else
            return "0 B";

        string[] suffixes = { "B", "KB", "MB", "GB", "TB" };
        int order = 0;
        double size = bytes;
        while (size >= 1024 && order < suffixes.Length - 1)
        {
            order++;
            size /= 1024;
        }
        return $"{size:0.##} {suffixes[order]}";
    }

    public object? ConvertBack(object? value, Type targetType, object? parameter, CultureInfo culture)
        => throw new NotImplementedException();
}
