using System;
using System.Globalization;
using Avalonia.Data.Converters;
using Avalonia.Media;

namespace PublishGui.Converters;

public class StatusToColorConverter : IValueConverter
{
    public object? Convert(object? value, Type targetType, object? parameter, CultureInfo culture)
    {
        var status = value as string;
        return status switch
        {
            "new" => new SolidColorBrush(Color.Parse("#22C55E")),
            "modified" => new SolidColorBrush(Color.Parse("#F59E0B")),
            "deleted" => new SolidColorBrush(Color.Parse("#EF4444")),
            _ => new SolidColorBrush(Color.Parse("#6B7280"))
        };
    }

    public object? ConvertBack(object? value, Type targetType, object? parameter, CultureInfo culture)
        => throw new NotSupportedException();
}