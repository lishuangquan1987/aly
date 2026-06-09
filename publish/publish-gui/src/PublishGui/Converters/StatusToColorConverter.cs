using System;
using System.Globalization;
using Avalonia.Data.Converters;
using Avalonia.Media;

namespace PublishGui.Converters;

public class StatusToColorConverter : IValueConverter
{
    private static readonly SolidColorBrush NewBrush = new(Color.Parse("#22C55E"));
    private static readonly SolidColorBrush ModifiedBrush = new(Color.Parse("#F59E0B"));
    private static readonly SolidColorBrush DeletedBrush = new(Color.Parse("#EF4444"));
    private static readonly SolidColorBrush DefaultBrush = new(Color.Parse("#6B7280"));

    public object? Convert(object? value, Type targetType, object? parameter, CultureInfo culture)
    {
        var status = value as string;
        return status switch
        {
            "new" => NewBrush,
            "modified" => ModifiedBrush,
            "deleted" => DeletedBrush,
            _ => DefaultBrush
        };
    }

    public object? ConvertBack(object? value, Type targetType, object? parameter, CultureInfo culture)
        => throw new NotSupportedException();
}