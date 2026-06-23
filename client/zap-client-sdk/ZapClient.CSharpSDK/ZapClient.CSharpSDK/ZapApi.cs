using Newtonsoft.Json;
using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using ZapClient.CSharpSDK.Models;

namespace ZapClient.CSharpSDK
{
    public class ZapApi
    {
        // ══════════════════════════════════════════════════
        //  公开 API
        // ══════════════════════════════════════════════════

        /// <summary>检查更新程序自身是否需要更新</summary>
        public static Task<ZapResponse<CheckSelfUpdateData>> CheckSelfUpdateAsync(string zapExePath)
            => RunAsync<CheckSelfUpdateData>("check_self_update", zapExePath);

        /// <summary>检查应用是否有新版本可更新</summary>
        public static Task<ZapResponse<CheckUpdateData>> CheckUpdateAsync(string zapExePath)
            => RunAsync<CheckUpdateData>("check_update", zapExePath);

        /// <summary>下载差异文件到本地</summary>
        public static Task<ZapResponse> DownloadUpdateAsync(string zapExePath, Action<string, double> progress)
        {
            return Task.Factory.StartNew(() =>
            {
                try
                {
                    var psi = new ProcessStartInfo
                    {
                        FileName = zapExePath,
                        Arguments = "download_update",
                        UseShellExecute = false,
                        RedirectStandardOutput = true,
                        RedirectStandardError = true,
                        CreateNoWindow = true,
                        WorkingDirectory = Path.GetDirectoryName(zapExePath),
                        StandardOutputEncoding = Encoding.UTF8,
                        StandardErrorEncoding = Encoding.UTF8
                    };

                    using (var cts = new CancellationTokenSource())
                    using (var process = new Process { StartInfo = psi })
                    {
                        process.Start();
                        process.Exited += (s, e) => cts.Cancel();

                        int doneCount = 0;
                        while (!cts.IsCancellationRequested)
                        {
                            var stdoutTask = process.StandardOutput.ReadLine();
                            if (!string.IsNullOrEmpty(stdoutTask))
                            {
                                var result = JsonConvert.DeserializeObject<ZapResponse<DownloadProgressData>>(stdoutTask);
                                if (!result.IsSuccess)
                                {
                                    return ZapResponse.NG(result.ErrorMsg);
                                }

                                if (result.Data == null)// finish
                                {
                                    return ZapResponse.OK();
                                }
                                // Count DONE + SKIP as completed files for accurate progress
                                if (result.Data.Status == "DONE" || result.Data.Status == "SKIP")
                                {
                                    doneCount++;
                                    progress?.Invoke(result.Data.File, doneCount / (double)result.Data.Total);
                                }
                            }
                        }
                    }

                    return ZapResponse.NG("Download interrupted");
                }
                catch (Exception ex)
                {
                    return ZapResponse.NG(ex);
                }
            });
        }

        /// <summary>应用更新（原子替换），会关闭主进程并重启。返回 OperationResult（无 data）。</summary>
        public static ZapResponse ApplyUpdateAsync(
            string zapExePath,
            string mustCloseProcessNames = null,
            int closeTimeoutSeconds = 30)
        {
            var args = "apply_update";
            if (!string.IsNullOrWhiteSpace(mustCloseProcessNames))
                args += $" --must-close-process-name \"{mustCloseProcessNames}\"";
            if (closeTimeoutSeconds != 30)
                args += $" --close-timeout {closeTimeoutSeconds}";

            return RunAsyncAlone(args, zapExePath);
        }

        /// <summary>获取可回滚的版本列表</summary>
        public static Task<ZapResponse<ListRollbackData>> ListRollbackVersionsAsync(string zapExePath)
            => RunAsync<ListRollbackData>("list_rollback_versions", zapExePath);

        /// <summary>回滚到指定版本</summary>
        public static Task<ZapResponse<RollbackData>> RollbackAsync(
            string zapExePath,
            string version,
            string mustCloseProcessNames = null,
            int closeTimeoutSeconds = 30)
        {
            var args = $"rollback --version {version}";
            if (!string.IsNullOrWhiteSpace(mustCloseProcessNames))
                args += $" --must-close-process-name \"{mustCloseProcessNames}\"";
            if (closeTimeoutSeconds != 30)
                args += $" --close-timeout {closeTimeoutSeconds}";

            return RunAsync<RollbackData>(args, zapExePath);
        }

        // ══════════════════════════════════════════════════
        //  内部
        // ══════════════════════════════════════════════════
        private static Task<ZapResponse<T>> RunAsync<T>(string arguments, string zapExePath)
        {
            return Task.Factory.StartNew(() =>
            {
                try
                {
                    var psi = new ProcessStartInfo
                    {
                        FileName = zapExePath,
                        Arguments = arguments,
                        UseShellExecute = false,
                        RedirectStandardOutput = true,
                        RedirectStandardError = true,
                        CreateNoWindow = true,
                        WorkingDirectory = Path.GetDirectoryName(zapExePath),
                        StandardOutputEncoding = Encoding.UTF8,
                        StandardErrorEncoding = Encoding.UTF8
                    };


                    using (var process = new Process { StartInfo = psi })
                    {
                        process.Start();

                        var stdoutTask = Task.Factory.StartNew(() => process.StandardOutput.ReadToEnd());
                        var stderrTask = Task.Factory.StartNew(() => process.StandardError.ReadToEnd());
                        Task.WaitAll(stdoutTask, stderrTask);

                        var stdout = stdoutTask.Result;
                        var stderr = stderrTask.Result;

                        if (!process.HasExited)
                        {
                            process.WaitForExit(5000);
                        }

                        if (process.ExitCode != 0 && string.IsNullOrWhiteSpace(stdout))
                        {
                            var err = stderr ?? "";
                            if (string.IsNullOrWhiteSpace(err))
                                err = "zap-client.exe exited with code " + process.ExitCode + " (may require admin)";
                            return ZapResponse<T>.NG(err);
                        }

                        if (string.IsNullOrWhiteSpace(stdout))
                        {
                            return ZapResponse<T>.NG(stderr ?? "zap-client.exe returned no output (may require admin)");
                        }

                        return JsonConvert.DeserializeObject<ZapResponse<T>>(stdout);
                    }
                }
                catch (Exception ex)
                {
                    return ZapResponse<T>.NG(ex);
                }
            });

        }

        /// <summary>
        /// 独立进程运行
        /// </summary>
        /// <param name="arguments"></param>
        /// <param name="zapExePath"></param>
        /// <returns></returns>
        private static ZapResponse RunAsyncAlone(string arguments, string zapExePath)
        {
            try
            {
                var psi = new ProcessStartInfo
                {
                    FileName = zapExePath,
                    Arguments = arguments,
                    UseShellExecute = true,
                    WorkingDirectory = Path.GetDirectoryName(zapExePath),
                };

                Process.Start(psi);
                return ZapResponse.OK();
            }
            catch (Exception ex)
            {
                return ZapResponse.NG(ex);
            }
        }
    }
}
