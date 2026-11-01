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

        /// <summary>
        /// 检查应用是否有新版本可更新。服务器不可达时返回 has_update=false
        /// （即使本机已有已下载的待应用版本：无法与服务器核对就不上报，联网后自动继续 apply）。
        /// </summary>
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
                    using (var process = new Process { StartInfo = psi, EnableRaisingEvents = true })
                    {
                        // Exited 事件依赖 EnableRaisingEvents（构造时已设），且必须在 Start 前订阅，
                        // 否则进程先退出时事件丢失（#1b）。
                        process.Exited += (s, e) => cts.Cancel();
                        process.Start();

                        while (!cts.IsCancellationRequested)
                        {
                            var stdoutTask = process.StandardOutput.ReadLine();
                            if (stdoutTask == null)
                            {
                                // EOF：client 进程已退出但未输出 data:null 完成行
                                //（崩溃/被杀/输出了 SDK 不识别的成功行）。必须终止循环，
                                // 否则 ReadLine 持续返回 null 导致 100% CPU 空转 + 永久挂起（#1a）。
                                return AlyResponse.NG("download interrupted (aly-client exited before finish line)");
                            }
                            if (stdoutTask.Length == 0)
                            {
                                continue; // 空行跳过
                            }

                            var result = JsonConvert.DeserializeObject<AlyResponse<DownloadProgressData>>(stdoutTask);
                            if (!result.IsSuccess)
                            {
                                return AlyResponse.NG(result.ErrorMsg);
                            }

                            if (result.Data == null)// finish
                            {
                                return AlyResponse.OK();
                            }
                            // 只统计 DONE（客户端已不再输出 SKIP），百分比 = 已完成序号/总数，
                            // 避免把未下载的文件算进进度导致界面统计失误
                            if (result.Data.Status == "DONE" && result.Data.Total > 0)
                            {
                                progress?.Invoke(result.Data.File, result.Data.Index / (double)result.Data.Total);
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
                            if (!process.WaitForExit(30000))
                            {
                                // 超时后进程仍存活：直接判失败返回，不访问 ExitCode
                                //（未退出进程访问 ExitCode 会抛 InvalidOperationException，#8）。
                                try { process.Kill(); } catch { }
                                return AlyResponse<T>.NG("aly-client.exe timed out after 30s (no exit)");
                            }
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
