using System.Diagnostics;

namespace PublishTool.Services;

public class ProcessService
{
    public void StartProcess(string exePath, string? arguments = null)
    {
        var psi = new ProcessStartInfo
        {
            FileName = exePath,
            Arguments = arguments ?? string.Empty,
            UseShellExecute = true
        };
        Process.Start(psi);
    }

    public void OpenFolder(string folderPath)
    {
        Process.Start(new ProcessStartInfo
        {
            FileName = folderPath,
            UseShellExecute = true
        });
    }

    public void OpenInExplorer(string filePath)
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
            process.Arguments = $"\"{filePath}\"";
        }
        Process.Start(process);
    }
}
