using Avalonia;
using Avalonia.Media;
using Avalonia.Styling;
using Avalonia.Data.Converters;
using PublishTool.Models.Local;
using System.Globalization;

namespace PublishTool.Converters;

public class UploadStatusToTextConverter : IValueConverter
{
    public object? Convert(object? value, Type targetType, object? parameter, CultureInfo culture)
    {
        if (value is UploadStatus status)
        {
            return status switch
            {
                UploadStatus.Pending => "等待",
                UploadStatus.Uploading => "上传中",
                UploadStatus.Done => "已完成",
                UploadStatus.Failed => "失败",
                _ => ""
            };
        }
        return "";
    }

    public object? ConvertBack(object? value, Type targetType, object? parameter, CultureInfo culture)
        => throw new NotImplementedException();
}

public class UploadStatusToColorConverter : IValueConverter
{
    public object? Convert(object? value, Type targetType, object? parameter, CultureInfo culture)
    {
        if (value is UploadStatus status)
        {
            var key = status switch
            {
                UploadStatus.Pending => "TextTertiary",
                UploadStatus.Uploading => "PrimaryColor",
                UploadStatus.Done => "SuccessColor",
                UploadStatus.Failed => "ErrorColor",
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