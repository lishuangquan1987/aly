using Avalonia.Controls;
using Avalonia.Media;

namespace ZapPublish.Helpers;

public enum StatusKind { Info, Error, Success, Warning }

public static class DialogHelper
{
    public static IBrush GetResourceBrush(string key, IBrush fallback)
    {
        if (Avalonia.Application.Current?.TryFindResource(key, out var value) == true && value is IBrush brush)
            return brush;
        return fallback;
    }

    public static void SetStatus(TextBlock statusText, string message, StatusKind kind,
        Avalonia.Controls.Panel? statusPanel = null)
    {
        statusText.Text = message;
        statusText.Foreground = kind switch
        {
            StatusKind.Error => GetResourceBrush("SemiColorDanger", Brushes.Red),
            StatusKind.Success => GetResourceBrush("SemiColorSuccess", Brushes.Green),
            StatusKind.Warning => GetResourceBrush("SemiColorWarning", Brushes.Orange),
            _ => GetResourceBrush("SemiColorText2", Brushes.Gray)
        };
        if (statusPanel != null) statusPanel.IsVisible = true;
    }
}
