using System.Diagnostics;
using System.Text;
using AlyPublish.Localization;
using Serilog;

namespace AlyPublish.Services;

public sealed record ProcessResult(
    bool Success,
    bool Canceled,
    int ExitCode,
    string StandardOutput,
    string StandardError);

/// <summary>Runs external processes with UTF-8 output, a timeout and full process-tree cleanup.</summary>
public static class ProcessService
{
    private const int KillWaitMilliseconds = 5_000;
    private const int DefaultTimeoutMilliseconds = 60_000;

    public static Task<ProcessResult> RunAsync(
        string fileName,
        IReadOnlyList<string> arguments,
        string? workingDirectory = null,
        int timeoutMs = DefaultTimeoutMilliseconds,
        CancellationToken cancellationToken = default)
        => RunCoreAsync(fileName, arguments, null, workingDirectory, timeoutMs, cancellationToken);

    public static Task<ProcessResult> RunWithProgressAsync(
        string fileName,
        IReadOnlyList<string> arguments,
        Action<string> onLine,
        string? workingDirectory = null,
        int timeoutMs = DefaultTimeoutMilliseconds,
        CancellationToken cancellationToken = default)
        => RunCoreAsync(fileName, arguments, onLine, workingDirectory, timeoutMs, cancellationToken);

    private static async Task<ProcessResult> RunCoreAsync(
        string fileName,
        IReadOnlyList<string> arguments,
        Action<string>? onLine,
        string? workingDirectory,
        int timeoutMs,
        CancellationToken cancellationToken)
    {
        var startInfo = CreateStartInfo(fileName, arguments, workingDirectory);
        Log.Debug("启动进程: {File} {Arguments} (超时={Timeout}ms)", fileName, string.Join(' ', arguments), timeoutMs);

        using var process = new Process { StartInfo = startInfo };
        try
        {
            process.Start();
        }
        catch (Exception ex)
        {
            Log.Error(ex, "进程启动失败: {File}", fileName);
            return new ProcessResult(false, false, -1, string.Empty, ex.Message);
        }

        var output = new StringBuilder();
        var outputTask = onLine is null
            ? ReadAllAsync(process.StandardOutput, output)
            : ReadLinesAsync(process.StandardOutput, output, onLine);
        var errorTask = process.StandardError.ReadToEndAsync(CancellationToken.None);

        using var timeoutSource = new CancellationTokenSource(timeoutMs);
        using var linkedSource = CancellationTokenSource.CreateLinkedTokenSource(timeoutSource.Token, cancellationToken);
        try
        {
            await process.WaitForExitAsync(linkedSource.Token);
        }
        catch (OperationCanceledException)
        {
            var canceledByCaller = cancellationToken.IsCancellationRequested && !timeoutSource.IsCancellationRequested;
            await TerminateAsync(process, fileName);
            await DrainOutputAsync(outputTask, errorTask);
            if (canceledByCaller)
            {
                Log.Information("进程已取消: {File}", fileName);
                return new ProcessResult(false, true, -1, output.ToString().Trim(), LanguageManager.T(LanguageManager.Instance.Proc_Cancelled));
            }

            Log.Warning("进程超时: {File} ({Timeout}ms)", fileName, timeoutMs);
            return new ProcessResult(false, false, -1, output.ToString().Trim(), LanguageManager.T(LanguageManager.Instance.Proc_Timeout));
        }

        await outputTask;
        var standardError = (await errorTask).Trim();
        var exitCode = process.ExitCode;
        var result = new ProcessResult(exitCode == 0, false, exitCode, output.ToString().Trim(), standardError);

        if (!result.Success)
        {
            Log.Warning("进程非零退出: ExitCode={ExitCode}, StdErr={Error}", exitCode, standardError);
        }
        else
        {
            Log.Debug("进程完成: ExitCode={ExitCode}, StdOut={Length}", exitCode, result.StandardOutput.Length);
        }

        return result;
    }

    private static ProcessStartInfo CreateStartInfo(
        string fileName,
        IReadOnlyList<string> arguments,
        string? workingDirectory)
    {
        var startInfo = new ProcessStartInfo
        {
            FileName = fileName,
            UseShellExecute = false,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            CreateNoWindow = true,
            StandardOutputEncoding = Encoding.UTF8,
            StandardErrorEncoding = Encoding.UTF8
        };

        foreach (var argument in arguments)
        {
            startInfo.ArgumentList.Add(argument);
        }

        if (!string.IsNullOrWhiteSpace(workingDirectory))
        {
            startInfo.WorkingDirectory = workingDirectory;
        }

        return startInfo;
    }

    private static async Task ReadAllAsync(StreamReader reader, StringBuilder output)
    {
        output.Append(await reader.ReadToEndAsync(CancellationToken.None));
    }

    private static async Task ReadLinesAsync(StreamReader reader, StringBuilder output, Action<string> onLine)
    {
        while (await reader.ReadLineAsync(CancellationToken.None) is { } line)
        {
            output.AppendLine(line);
            try
            {
                onLine(line);
            }
            catch (Exception ex)
            {
                Log.Warning(ex, "进度回调异常: {Line}", line);
            }
        }
    }

    private static async Task TerminateAsync(Process process, string fileName)
    {
        try
        {
            process.Kill(entireProcessTree: true);
        }
        catch (InvalidOperationException)
        {
            return;
        }

        try
        {
            using var killSource = new CancellationTokenSource(KillWaitMilliseconds);
            await process.WaitForExitAsync(killSource.Token);
        }
        catch (OperationCanceledException)
        {
            Log.Warning("进程 Kill 后未在 {Timeout}ms 内退出: {File}", KillWaitMilliseconds, fileName);
        }
    }

    private static async Task DrainOutputAsync(Task outputTask, Task<string> errorTask)
    {
        try
        {
            await Task.WhenAll(outputTask, errorTask);
        }
        catch (Exception ex)
        {
            Log.Warning(ex, "超时后读取进程输出失败");
        }
    }
}
