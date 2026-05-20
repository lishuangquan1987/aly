using Avalonia.Media;
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
            return status switch
            {
                FileCompareStatus.New => new SolidColorBrush(Color.Parse("#00a8ff")),
                FileCompareStatus.Modified => new SolidColorBrush(Color.Parse("#ff9500")),
                FileCompareStatus.Deleted => new SolidColorBrush(Color.Parse("#ff3b30")),
                _ => new SolidColorBrush(Color.Parse("#8e8e93"))
            };
        }
        return new SolidColorBrush(Color.Parse("#8e8e93"));
    }

    public object? ConvertBack(object? value, Type targetType, object? parameter, CultureInfo culture)
        => throw new NotImplementedException();
}