using Avalonia;
using Avalonia.Media;
using Avalonia.Styling;
using Avalonia.Data.Converters;
using PublishTool.Models.Local;
using System.Globalization;

namespace PublishTool.Converters;

public class StatusToColorConverter : IValueConverter
{
    public object? Convert(object? value, Type targetType, object? parameter, CultureInfo culture)
    {
        if (value is FileCompareStatus status)
        {
            var key = status switch
            {
                FileCompareStatus.New => "PrimaryColor",
                FileCompareStatus.Modified => "WarningColor",
                FileCompareStatus.Deleted => "ErrorColor",
                _ => "TextTertiary"
            };
            return ResolveBrush(key, Colors.Gray);
        }
        return ResolveBrush("TextTertiary", Colors.Gray);
    }

    public object? ConvertBack(object? value, Type targetType, object? parameter, CultureInfo culture)
        => throw new NotImplementedException();

    private static SolidColorBrush ResolveBrush(string resourceKey, Color fallback)
    {
        if (Application.Current?.TryGetResource(resourceKey, ThemeVariant.Default, out var resource) == true && resource is Color color)
            return new SolidColorBrush(color);
        return new SolidColorBrush(fallback);
    }
}