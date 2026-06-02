using Avalonia.Data.Converters;
using Avalonia.Media;
using System.Globalization;

namespace PublishGui.Converters;

public class StatusToColorConverter : IValueConverter
{
    public object? Convert(object? value, Type targetType, object? parameter, CultureInfo culture)
    {
        if (value is string status)
        {
            return status switch
            {
                "new" => new SolidColorBrush(Color.Parse("#52c41a")),
                "modified" => new SolidColorBrush(Color.Parse("#1890ff")),
                "deleted" => new SolidColorBrush(Color.Parse("#ff4d4f")),
                _ => new SolidColorBrush(Color.Parse("#8c8c8c"))
            };
        }
        return new SolidColorBrush(Color.Parse("#8c8c8c"));
    }

    public object? ConvertBack(object? value, Type targetType, object? parameter, CultureInfo culture)
        => throw new NotImplementedException();
}
