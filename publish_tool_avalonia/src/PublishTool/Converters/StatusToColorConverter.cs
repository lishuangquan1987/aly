using System;
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
                FileCompareStatus.New => "SemiColorPrimary",
                FileCompareStatus.Modified => "SemiColorWarning",
                FileCompareStatus.Deleted => "SemiColorDanger",
                _ => "SemiColorText2"
            };
            return BrushHelper.ResolveBrush(key, Avalonia.Media.Colors.Gray);
        }
        return BrushHelper.ResolveBrush("SemiColorText2", Avalonia.Media.Colors.Gray);
    }

    public object? ConvertBack(object? value, Type targetType, object? parameter, CultureInfo culture)
        => throw new NotImplementedException();
}
