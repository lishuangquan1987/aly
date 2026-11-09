using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Avalonia.Controls;
using Avalonia.Platform.Storage;
using Serilog;

namespace AlyPublish.Views.Dialogs;

/// <summary>Thin wrapper around the platform storage pickers used by the dialogs.</summary>
internal static class StoragePicker
{
    public static async Task<string?> PickFolderAsync(Window owner, string title, string? startFolder = null)
    {
        try
        {
            var folders = await owner.StorageProvider.OpenFolderPickerAsync(new FolderPickerOpenOptions
            {
                Title = title,
                AllowMultiple = false,
                SuggestedStartLocation = await TryGetFolderAsync(owner, startFolder)
            });
            return folders.Count > 0 ? folders[0].TryGetLocalPath() : null;
        }
        catch (Exception ex)
        {
            Log.Error(ex, "打开文件夹选择器失败");
            return null;
        }
    }

    public static async Task<IReadOnlyList<string>> PickFoldersAsync(
        Window owner,
        string title,
        string? startFolder = null)
    {
        try
        {
            var folders = await owner.StorageProvider.OpenFolderPickerAsync(new FolderPickerOpenOptions
            {
                Title = title,
                AllowMultiple = true,
                SuggestedStartLocation = await TryGetFolderAsync(owner, startFolder)
            });

            return folders
                .Select(folder => folder.TryGetLocalPath())
                .Where(path => !string.IsNullOrWhiteSpace(path))
                .Select(path => path!)
                .ToList();
        }
        catch (Exception ex)
        {
            Log.Error(ex, "打开文件夹选择器失败");
            return [];
        }
    }

    public static async Task<IReadOnlyList<string>> PickFilesAsync(
        Window owner,
        string title,
        string? startFolder = null)
    {
        try
        {
            var files = await owner.StorageProvider.OpenFilePickerAsync(new FilePickerOpenOptions
            {
                Title = title,
                AllowMultiple = true,
                SuggestedStartLocation = await TryGetFolderAsync(owner, startFolder)
            });

            return files
                .Select(file => file.TryGetLocalPath())
                .Where(path => !string.IsNullOrWhiteSpace(path))
                .Select(path => path!)
                .ToList();
        }
        catch (Exception ex)
        {
            Log.Error(ex, "打开文件选择器失败");
            return [];
        }
    }

    public static async Task<string?> PickExecutableAsync(
        Window owner,
        string title,
        string? startFolder = null)
    {
        try
        {
            var files = await owner.StorageProvider.OpenFilePickerAsync(new FilePickerOpenOptions
            {
                Title = title,
                AllowMultiple = false,
                SuggestedStartLocation = await TryGetFolderAsync(owner, startFolder),
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

    private static async Task<IStorageFolder?> TryGetFolderAsync(Window owner, string? path)
    {
        if (string.IsNullOrWhiteSpace(path))
        {
            return null;
        }

        try
        {
            return await owner.StorageProvider.TryGetFolderFromPathAsync(new Uri(Path.GetFullPath(path)));
        }
        catch (Exception ex)
        {
            Log.Debug(ex, "无法定位选择器起始目录: {Path}", path);
            return null;
        }
    }
}
