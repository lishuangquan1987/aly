using System;
using System.Diagnostics;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using Serilog;

namespace ZapPublish.Services;

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
        if (!string.IsNullOrWhiteSpace(workingDir))
        {
            psi.WorkingDirectory = workingDir;
            Log.Debug("工作目录: {WorkDir}", workingDir);
        }

        Log.Debug("启动进程: {File} {Args} (超时={Timeout}ms)", fileName, arguments, timeoutMs);

        using var proc = new Process { StartInfo = psi };

        try
        {
            proc.Start();

            using var cts = new CancellationTokenSource(timeoutMs);
            try
            {
                await proc.WaitForExitAsync(cts.Token);
            }
            catch (OperationCanceledException)
            {
                proc.Kill();
                await proc.WaitForExitAsync(CancellationToken.None);
                Log.Warning("进程超时: {File} {Args}", fileName, arguments);
                return new ProcessResult { Success = false, StandardError = "进程执行超时" };
            }

            // 进程已退出，文件流已关闭，此时同步读取不会死锁
            var stdout = proc.StandardOutput.ReadToEnd();
            var stderr = proc.StandardError.ReadToEnd();

            var result = new ProcessResult
            {
                Success = proc.ExitCode == 0,
                StandardOutput = stdout.Trim(),
                StandardError = stderr.Trim(),
                ExitCode = proc.ExitCode
            };

            Log.Debug("进程完成: ExitCode={Code}, StdOut={OutLen}, StdErr={ErrLen}",
                result.ExitCode, result.StandardOutput.Length, result.StandardError.Length);

            if (!result.Success)
            {
                Log.Warning("进程非零退出: ExitCode={Code}, StdErr={Err}",
                    result.ExitCode, result.StandardError);
            }

            return result;
        }
        catch (Exception ex)
        {
            Log.Error(ex, "进程执行异常: {File} {Args}", fileName, arguments);
            return new ProcessResult { Success = false, StandardError = ex.Message };
        }
    }
}
