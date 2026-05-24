using Avalonia;
using Avalonia.Media;
using Avalonia.Styling;

namespace PublishTool.Converters;

internal static class BrushHelper
{
    public static SolidColorBrush ResolveBrush(string resourceKey, Color fallback)
    {
        if (Application.Current?.TryGetResource(resourceKey, ThemeVariant.Default, out var resource) == true
            && resource is Color color)
            return new SolidColorBrush(color);
        return new SolidColorBrush(fallback);
    }
}
