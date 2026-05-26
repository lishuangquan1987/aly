using System;
using Avalonia.Data.Converters;
using System.Globalization;

namespace PublishTool.Converters;

public class BoolToColorConverter : IValueConverter
{
    public object? Convert(object? value, Type targetType, object? parameter, CultureInfo culture)
    {
        if (value is bool b && b)
            return BrushHelper.ResolveBrush("SemiColorSuccess", Avalonia.Media.Colors.Green);
        return BrushHelper.ResolveBrush("SemiColorText2", Avalonia.Media.Colors.Gray);
    }

    public object? ConvertBack(object? value, Type targetType, object? parameter, CultureInfo culture)
        => throw new NotImplementedException();
}
