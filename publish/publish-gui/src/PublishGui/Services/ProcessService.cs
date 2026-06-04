using System;
using System.Diagnostics;
using System.Text;
using System.Threading.Tasks;
using Serilog;

namespace PublishGui.Services;

public class ProcessResult
{
    public bool Success { get; set; }
    public string StandardOutput { get; set; } = string.Empty;
    public string StandardError { get; set; } = string.Empty;
    public int ExitCode { get; set; }
}

public class ProcessService
{
    public async Task<ProcessResult> RunAsync(
        string fileName, string arguments, string? workingDir = null, int timeoutMs = 30000)
    {
        var sbOut = new StringBuilder();
        var sbErr = new StringBuilder();

        var psi = new ProcessStartInfo
        {
            FileName = fileName,
            Arguments = arguments,
            UseShellExecute = false,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            CreateNoWindow = true,
            StandardOutputEncoding = Encoding.UTF8,
            StandardErrorEncoding = Encoding.UTF8
        };
        if (workingDir != null) psi.WorkingDirectory = workingDir;

        using var proc = new Process { StartInfo = psi };

        proc.OutputDataReceived += (_, e) => { if (e.Data != null) sbOut.AppendLine(e.Data); };
        proc.ErrorDataReceived += (_, e) => { if (e.Data != null) sbErr.AppendLine(e.Data); };

        try
        {
            proc.Start();
            proc.BeginOutputReadLine();
            proc.BeginErrorReadLine();

            if (!proc.WaitForExit(timeoutMs))
            {
                proc.Kill();
                return new ProcessResult { Success = false, StandardError = "Process timed out" };
            }

            // Wait for async output to complete
            proc.WaitForExit();

            return new ProcessResult
            {
                Success = proc.ExitCode == 0,
                StandardOutput = sbOut.ToString().Trim(),
                StandardError = sbErr.ToString().Trim(),
                ExitCode = proc.ExitCode
            };
        }
        catch (Exception ex)
        {
            Log.Error(ex, "Process {File} {Args} failed", fileName, arguments);
            return new ProcessResult { Success = false, StandardError = ex.Message };
        }
    }
}