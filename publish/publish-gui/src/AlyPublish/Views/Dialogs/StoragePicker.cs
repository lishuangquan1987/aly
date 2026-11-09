using System.Threading.Tasks;
using Avalonia.Controls;
using Avalonia.Platform.Storage;
using Serilog;

namespace AlyPublish.Views.Dialogs;

/// <summary>Thin wrapper around the platform storage pickers used by the dialogs.</summary>
internal static class StoragePicker
{
    public static async Task<string?> PickFolderAsync(Window owner, string title)
    {
        try
        {
            var folders = await owner.StorageProvider.OpenFolderPickerAsync(new FolderPickerOpenOptions
            {
                Title = title,
                AllowMultiple = false
            });
            return folders.Count > 0 ? folders[0].TryGetLocalPath() : null;
        }
        catch (Exception ex)
        {
            Log.Error(ex, "打开文件夹选择器失败");
            return null;
        }
    }

    public static async Task<string?> PickExecutableAsync(Window owner, string title)
    {
        try
        {
            var files = await owner.StorageProvider.OpenFilePickerAsync(new FilePickerOpenOptions
            {
                Title = title,
                AllowMultiple = false,
                FileTypeFilter =
                [
                    new FilePickerFileType("可执行文件")
                    {
                        Patterns = ["*.exe"]
                    }
                ]
            });
            return files.Count > 0 ? files[0].TryGetLocalPath() : null;
        }
        catch (Exception ex)
        {
            Log.Error(ex, "打开文件选择器失败");
            return null;
        }
    }
}
