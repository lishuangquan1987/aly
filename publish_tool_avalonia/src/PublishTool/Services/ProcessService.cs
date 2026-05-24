using System.Diagnostics;
using Serilog;

namespace PublishTool.Services;

public class ProcessService
{
    public void StartProcess(string exePath, string? arguments = null)
    {
        try
        {
            var psi = new ProcessStartInfo
            {
                FileName = exePath,
                Arguments = arguments ?? string.Empty,
                UseShellExecute = true
            };
            Process.Start(psi);
            Log.Information("启动进程: {ExePath} {Arguments}", exePath, arguments ?? string.Empty);
        }
        catch (Exception ex)
        {
            Log.Error(ex, "启动进程失败: {ExePath}", exePath);
            throw;
        }
    }

    public void OpenFolder(string folderPath)
    {
        try
        {
            Process.Start(new ProcessStartInfo
            {
                FileName = folderPath,
                UseShellExecute = true
            });
            Log.Information("打开文件夹: {Path}", folderPath);
        }
        catch (Exception ex)
        {
            Log.Error(ex, "打开文件夹失败: {Path}", folderPath);
            throw;
        }
    }

    public void OpenInExplorer(string filePath)
    {
        try
        {
            var process = new ProcessStartInfo();
            if (OperatingSystem.IsWindows())
            {
                process.FileName = "explorer.exe";
                process.Arguments = $"/select,\"{filePath}\"";
            }
            else if (OperatingSystem.IsMacOS())
            {
                process.FileName = "open";
                process.Arguments = $"-R \"{filePath}\"";
            }
            else if (OperatingSystem.IsLinux())
            {
                process.FileName = "xdg-open";
                process.Arguments = $"\"{Path.GetDirectoryName(filePath)}\"";
            }
            Process.Start(process);
            Log.Information("在资源管理器中打开: {Path}", filePath);
        }
        catch (Exception ex)
        {
            Log.Error(ex, "在资源管理器中打开失败: {Path}", filePath);
            throw;
        }
    }
}
