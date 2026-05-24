using Avalonia.Data.Converters;
using System.Globalization;

namespace PublishTool.Converters;

public class DiskInfoFormatterConverter : IMultiValueConverter
{
    public object? Convert(IList<object?> values, Type targetType, object? parameter, CultureInfo culture)
    {
        if (values.Count >= 3 &&
            values[0] is double diskUsed &&
            values[1] is double diskTotal &&
            values[2] is double diskPercent)
        {
            return $"{diskUsed:F1} / {diskTotal:F1} GB ({diskPercent:F0}%)";
        }
        return "-";
    }
}
