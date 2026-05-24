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
                UploadStatus.Pending => "待上传",
                UploadStatus.Uploading => "上传中",
                UploadStatus.Done => "完成",
                UploadStatus.Failed => "失败",
                _ => ""
            };
        }
        return "";
    }

    public object? ConvertBack(object? value, Type targetType, object? parameter, CultureInfo culture)
        => throw new NotImplementedException();
}
