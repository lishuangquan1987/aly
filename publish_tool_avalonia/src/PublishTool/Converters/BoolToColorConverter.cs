using Avalonia;
using Avalonia.Media;
using Avalonia.Styling;
using Avalonia.Data.Converters;
using System.Globalization;

namespace PublishTool.Converters;

public class BoolToColorConverter : IValueConverter
{
    public object? Convert(object? value, Type targetType, object? parameter, CultureInfo culture)
    {
        var key = value is bool b && b ? "SemiColorSuccess" : "SemiColorText2";
        if (Application.Current?.TryGetResource(key, ThemeVariant.Default, out var resource) == true && resource is Color color)
            return new SolidColorBrush(color);
        return value is bool b2 && b2 ? new SolidColorBrush(Colors.Green) : new SolidColorBrush(Colors.Gray);
    }

    public object? ConvertBack(object? value, Type targetType, object? parameter, CultureInfo culture)
        => throw new NotImplementedException();
}
