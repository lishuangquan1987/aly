using Avalonia.Data.Converters;
using PublishTool.Models;
using System.Globalization;

namespace PublishTool.Converters;

public class DiskInfoTooltipConverter : IValueConverter
{
    public object? Convert(object? value, Type targetType, object? parameter, CultureInfo culture)
    {
        if (value is ServerOsInfoDto serverOsInfo)
        {
            return $"已用: {serverOsInfo.DiskUsed:F2} GB\n可用: {serverOsInfo.DiskFree:F2} GB\n总计: {serverOsInfo.DiskTotal:F2} GB";
        }
        return "磁盘信息";
    }

    public object? ConvertBack(object? value, Type targetType, object? parameter, CultureInfo culture)
        => throw new NotImplementedException();
}
