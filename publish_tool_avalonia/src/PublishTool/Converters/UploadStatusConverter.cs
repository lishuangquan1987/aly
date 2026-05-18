using Avalonia.Data.Converters;
using System.Globalization;

namespace PublishTool.Converters;

public class UploadStatusConverter : IValueConverter
{
    public object? Convert(object? value, Type targetType, object? parameter, CultureInfo culture)
    {
        if (value is PublishTool.Models.Local.UploadStatus status)
        {
            return status switch
            {
                PublishTool.Models.Local.UploadStatus.Pending => "待上传",
                PublishTool.Models.Local.UploadStatus.Uploading => "上传中",
                PublishTool.Models.Local.UploadStatus.Done => "完成",
                PublishTool.Models.Local.UploadStatus.Failed => "失败",
                _ => status.ToString()
            };
        }
        return value?.ToString();
    }

    public object? ConvertBack(object? value, Type targetType, object? parameter, CultureInfo culture)
        => throw new NotImplementedException();
}
