using Newtonsoft.Json;
using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using AlyClient.CSharpSDK.Models;

namespace AlyClient.CSharpSDK
{
    public class AlyApi
    {
        // ══════════════════════════════════════════════════
        //  公开 API
        // ══════════════════════════════════════════════════

        /// <summary>检查更新程序自身是否需要更新</summary>
        public static Task<AlyResponse<CheckSelfUpdateData>> CheckSelfUpdateAsync(string alyExePath)
            => RunAsync<CheckSelfUpdateData>("check_self_update", alyExePath);

        /// <summary>检查应用是否有新版本可更新</summary>
        public static Task<AlyResponse<CheckUpdateData>> CheckUpdateAsync(string alyExePath)
            => RunAsync<CheckUpdateData>("check_update", alyExePath);

        /// <summary>下载差异文件到本地</summary>
        public static Task<AlyResponse> DownloadUpdateAsync(string alyExePath, Action<string, double> progress)
        {
            return Task.Factory.StartNew(() =>
            {
                try
                {
                    var psi = new ProcessStartInfo
                    {
                        FileName = alyExePath,
                        Arguments = "download_update",
                        UseShellExecute = false,
                        RedirectStandardOutput = true,
                        RedirectStandardError = true,
                        CreateNoWindow = true,
                        WorkingDirectory = Path.GetDirectoryName(alyExePath),
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
                                var result = JsonConvert.DeserializeObject<AlyResponse<DownloadProgressData>>(stdoutTask);
                                if (!result.IsSuccess)
                                {
                                    return AlyResponse.NG(result.ErrorMsg);
                                }

                                if (result.Data == null)// finish
                                {
                                    return AlyResponse.OK();
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

                    return AlyResponse.NG("Download interrupted");
                }
                catch (Exception ex)
                {
                    return AlyResponse.NG(ex);
                }
            });
        }

        /// <summary>应用更新（原子替换），会关闭主进程并重启。返回 OperationResult（无 data）。</summary>
        public static AlyResponse ApplyUpdateAsync(
            string alyExePath,
            string mustCloseProcessNames = null,
            int closeTimeoutSeconds = 30)
        {
            var args = "apply_update";
            if (!string.IsNullOrWhiteSpace(mustCloseProcessNames))
                args += $" --must-close-process-name \"{mustCloseProcessNames}\"";
            if (closeTimeoutSeconds != 30)
                args += $" --close-timeout {closeTimeoutSeconds}";

            return RunAsyncAlone(args, alyExePath);
        }

        /// <summary>获取可回滚的版本列表</summary>
        public static Task<AlyResponse<ListRollbackData>> ListRollbackVersionsAsync(string alyExePath)
            => RunAsync<ListRollbackData>("list_rollback_versions", alyExePath);

        /// <summary>回滚到指定版本</summary>
        public static Task<AlyResponse<RollbackData>> RollbackAsync(
            string alyExePath,
            string version,
            string mustCloseProcessNames = null,
            int closeTimeoutSeconds = 30)
        {
            var args = $"rollback --version {version}";
            if (!string.IsNullOrWhiteSpace(mustCloseProcessNames))
                args += $" --must-close-process-name \"{mustCloseProcessNames}\"";
            if (closeTimeoutSeconds != 30)
                args += $" --close-timeout {closeTimeoutSeconds}";

            return RunAsync<RollbackData>(args, alyExePath);
        }

        // ══════════════════════════════════════════════════
        //  内部
        // ══════════════════════════════════════════════════
        private static Task<AlyResponse<T>> RunAsync<T>(string arguments, string alyExePath)
        {
            return Task.Factory.StartNew(() =>
            {
                try
                {
                    var psi = new ProcessStartInfo
                    {
                        FileName = alyExePath,
                        Arguments = arguments,
                        UseShellExecute = false,
                        RedirectStandardOutput = true,
                        RedirectStandardError = true,
                        CreateNoWindow = true,
                        WorkingDirectory = Path.GetDirectoryName(alyExePath),
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
                            process.WaitForExit(30000);
                        }

                        if (process.ExitCode != 0 && string.IsNullOrWhiteSpace(stdout))
                        {
                            var err = stderr ?? "";
                            if (string.IsNullOrWhiteSpace(err))
                                err = "aly-client.exe exited with code " + process.ExitCode + " (may require admin)";
                            return AlyResponse<T>.NG(err);
                        }

                        if (string.IsNullOrWhiteSpace(stdout))
                        {
                            return AlyResponse<T>.NG(stderr ?? "aly-client.exe returned no output (may require admin)");
                        }

                        return JsonConvert.DeserializeObject<AlyResponse<T>>(stdout);
                    }
                }
                catch (Exception ex)
                {
                    return AlyResponse<T>.NG(ex);
                }
            });

        }

        /// <summary>
        /// 独立进程运行
        /// </summary>
        /// <param name="arguments"></param>
        /// <param name="alyExePath"></param>
        /// <returns></returns>
        private static AlyResponse RunAsyncAlone(string arguments, string alyExePath)
        {
            try
            {
                var psi = new ProcessStartInfo
                {
                    FileName = alyExePath,
                    Arguments = arguments,
                    UseShellExecute = false,
                    CreateNoWindow = true,
                    WorkingDirectory = Path.GetDirectoryName(alyExePath),
                };

                Process.Start(psi);
                return AlyResponse.OK();
            }
            catch (Exception ex)
            {
                return AlyResponse.NG(ex);
            }
        }
    }
}
