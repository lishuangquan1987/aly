using System.Security.Cryptography;
using PublishTool.Models;
using PublishTool.Models.Local;
using Serilog;

namespace PublishTool.Services;

public class LocalFileService
{
    public List<LocalFileItem> ScanDirectory(
        string directoryPath,
        List<string>? ignoreFolders = null,
        List<string>? ignoreFiles = null)
    {
        var items = new List<LocalFileItem>();
        if (!Directory.Exists(directoryPath))
        {
            Log.Warning("目录不存在: {Path}", directoryPath);
            return items;
        }

        try
        {
            var folderSet = new HashSet<string>(
                ignoreFolders ?? Enumerable.Empty<string>(),
                StringComparer.OrdinalIgnoreCase);
            var fileSet = new HashSet<string>(
                ignoreFiles ?? Enumerable.Empty<string>(),
                StringComparer.OrdinalIgnoreCase);

            foreach (var file in Directory.GetFiles(directoryPath, "*", SearchOption.AllDirectories))
            {
                var fullPath = Path.GetFullPath(file);
                var relativePath = Path.GetRelativePath(directoryPath, fullPath);

                if (ShouldIgnore(relativePath, folderSet, fileSet))
                    continue;

                items.Add(new LocalFileItem
                {
                    FileName = Path.GetFileName(file),
                    AbsolutePath = fullPath,
                    RelativePath = relativePath,
                    LastModified = File.GetLastWriteTime(file),
                    IsChecked = false,
                    IsModified = false
                });
            }
        }
        catch (Exception ex)
        {
            Log.Error(ex, "扫描目录失败: {Path}", directoryPath);
        }

        return items;
    }

    private static bool ShouldIgnore(string relativePath, HashSet<string> folders, HashSet<string> files)
    {
        var parts = relativePath.Split(Path.DirectorySeparatorChar, Path.AltDirectorySeparatorChar);
        foreach (var part in parts)
        {
            if (folders.Contains(part))
                return true;
        }
        var fileName = Path.GetFileName(relativePath);
        if (files.Contains(fileName) || files.Contains("*" + Path.GetExtension(fileName)))
            return true;
        return false;
    }

    public string CalculateMd5(string filePath)
    {
        try
        {
            using var md5 = MD5.Create();
            using var stream = File.OpenRead(filePath);
            var hash = md5.ComputeHash(stream);
            return BitConverter.ToString(hash).Replace("-", "").ToLowerInvariant();
        }
        catch (Exception ex)
        {
            Log.Error(ex, "计算 MD5 失败: {Path}", filePath);
            return string.Empty;
        }
    }

    public async Task<string> CalculateMd5Async(string filePath, CancellationToken cancellationToken = default)
    {
        try
        {
            using var md5 = MD5.Create();
            await using var stream = File.OpenRead(filePath);
            var hash = await md5.ComputeHashAsync(stream, cancellationToken);
            return BitConverter.ToString(hash).Replace("-", "").ToLowerInvariant();
        }
        catch (Exception ex)
        {
            Log.Error(ex, "计算 MD5 失败: {Path}", filePath);
            return string.Empty;
        }
    }

    public List<LocalFileItem> GetModifiedFiles(
        string localPath,
        List<FileInfoDto> serverFiles,
        List<string>? ignoreFolders = null,
        List<string>? ignoreFiles = null)
    {
        var localFiles = ScanDirectory(localPath, ignoreFolders, ignoreFiles);
        var serverFileDict = serverFiles.ToDictionary(f => f.FileRelativePath);

        foreach (var local in localFiles)
        {
            if (serverFileDict.TryGetValue(local.RelativePath, out var serverFile))
            {
                var localMd5 = CalculateMd5(local.AbsolutePath);
                local.IsModified = !string.Equals(localMd5, serverFile.Md5,
                    StringComparison.OrdinalIgnoreCase);
            }
            else
            {
                local.IsModified = true;
            }
        }
        return localFiles;
    }
}
