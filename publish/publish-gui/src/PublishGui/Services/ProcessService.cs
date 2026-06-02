using Serilog;
using System;
using System.Diagnostics;
using System.Text;
using System.Threading;
using System.Threading.Tasks;

namespace PublishGui.Services;

public class ProcessResult
{
    public int ExitCode { get; set; }
    public string StandardOutput { get; set; } = string.Empty;
    public string StandardError { get; set; } = string.Empty;
    public bool Success => ExitCode == 0;
}

public class ProcessService
{
    public async Task<ProcessResult> RunAsync(
        string fileName,
        string arguments,
        string? workingDirectory = null,
        int timeoutMs = 30000,
        CancellationToken cancellationToken = default)
    {
        Log.Information("Running: {FileName} {Arguments}", fileName, arguments);

        var psi = new ProcessStartInfo
        {
            FileName = fileName,
            Arguments = arguments,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            UseShellExecute = false,
            CreateNoWindow = true
        };

        if (!string.IsNullOrEmpty(workingDirectory))
        {
            psi.WorkingDirectory = workingDirectory;
        }

        using var process = new Process { StartInfo = psi };
        
        var outputBuilder = new StringBuilder();
        var errorBuilder = new StringBuilder();

        process.OutputDataReceived += (_, e) =>
        {
            if (e.Data != null) outputBuilder.AppendLine(e.Data);
        };
        process.ErrorDataReceived += (_, e) =>
        {
            if (e.Data != null) errorBuilder.AppendLine(e.Data);
        };

        try
        {
            process.Start();
            process.BeginOutputReadLine();
            process.BeginErrorReadLine();

            using var cts = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
            cts.CancelAfter(timeoutMs);

            await process.WaitForExitAsync(cts.Token);

            var result = new ProcessResult
            {
                ExitCode = process.ExitCode,
                StandardOutput = outputBuilder.ToString().TrimEnd(),
                StandardError = errorBuilder.ToString().TrimEnd()
            };

            Log.Information("Process exited with {Code}, Output length: {OutputLen}", 
                result.ExitCode, result.StandardOutput.Length);

            return result;
        }
        catch (OperationCanceledException)
        {
            try { process.Kill(true); } catch { }
            return new ProcessResult
            {
                ExitCode = -1,
                StandardError = "Process timed out or was cancelled"
            };
        }
        catch (Exception ex)
        {
            Log.Error(ex, "Failed to run process: {FileName}", fileName);
            return new ProcessResult
            {
                ExitCode = -1,
                StandardError = ex.Message
            };
        }
    }
}
