using Avalonia.Media;
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
            return status switch
            {
                UploadStatus.Pending => new SolidColorBrush(Color.Parse("#8e8e93")),
                UploadStatus.Uploading => new SolidColorBrush(Color.Parse("#00a8ff")),
                UploadStatus.Done => new SolidColorBrush(Color.Parse("#30d158")),
                UploadStatus.Failed => new SolidColorBrush(Color.Parse("#ff3b30")),
                _ => new SolidColorBrush(Color.Parse("#8e8e93"))
            };
        }
        return new SolidColorBrush(Color.Parse("#8e8e93"));
    }

    public object? ConvertBack(object? value, Type targetType, object? parameter, CultureInfo culture)
        => throw new NotImplementedException();
}