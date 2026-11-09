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
        /// 执行更新器自更新（self_update）：按 SHA256 比较后，用主程序目录随版本
        /// 分发的 aly-client.exe 替换 UpdateFolder 中正在运行的副本。幂等；源缺失/
        /// 已是最新都返回 isSuccess=true。
        /// </summary>
        public static Task<AlyResponse<SelfUpdateData>> SelfUpdateAsync(string alyExePath)
            => RunAsync<SelfUpdateData>("self_update", alyExePath);

        /// <summary>
        /// 检查应用是否有新版本可更新。服务器不可达时返回 has_update=false
        /// （即使本机已有已下载的待应用版本：无法与服务器核对就不上报，联网后自动继续 apply）。
        /// </summary>
        public static Task<AlyResponse<CheckUpdateData>> CheckUpdateAsync(string alyExePath)
            => RunAsync<CheckUpdateData>("check_update", alyExePath);

        /// <summary>下载差异文件到本地</summary>
        public static Task<AlyResponse> DownloadUpdateAsync(string alyExePath, Action<string, double> progress)
        {
            return DownloadUpdateAsync(alyExePath, progress, DefaultDownloadStallTimeoutSeconds);
        }

        /// <summary>
        /// 下载停流看门狗默认超时（秒）。客户端现在对单文件大下载每 3 秒输出一行
        /// PROGRESS（见 download_update.go），因此"超过该时长无任何输出"即为真停流
        /// （网络挂起/磁盘卡死/进程僵死），而不是"大文件正常下载中"。
        /// 默认 1800s 给慢网络留足余量——真正的活性由 PROGRESS 行保证，不靠缩短超时。
        /// </summary>
        public const int DefaultDownloadStallTimeoutSeconds = 1800;

        /// <summary>
        /// 下载差异文件到本地（带停流看门狗）。
        /// stallTimeoutSeconds：无输出超时（秒）；超过即杀进程并返回失败。
        /// 旧版 ReadLine 无限阻塞：客户端真停流（非大文件正常下载）时 SDK 永久挂起、
        /// 宿主无法感知；配合客户端 3 秒 PROGRESS 行，该看门狗可以放心用长超时。
        /// </summary>
        public static Task<AlyResponse> DownloadUpdateAsync(string alyExePath, Action<string, double> progress, int stallTimeoutSeconds)
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

                        // 停流看门狗：距上一行输出超过 stallTimeout 即杀进程。
                        // Kill 后管道 EOF，ReadLine 返回 null，主循环走 EOF 分支收尾。
                        var lastLineAt = DateTime.UtcNow;
                        var stalled = false;
                        var watchdog = new System.Threading.Timer(_ =>
                        {
                            if ((DateTime.UtcNow - lastLineAt).TotalSeconds > stallTimeoutSeconds)
                            {
                                stalled = true;
                                try { process.Kill(); } catch { }
                            }
                        }, null, 5000, 5000);
                        using (watchdog)
                        {
                            while (!cts.IsCancellationRequested)
                            {
                                var stdoutTask = process.StandardOutput.ReadLine();
                                if (stdoutTask == null)
                                {
                                    // EOF：client 进程已退出但未输出 data:null 完成行
                                    //（崩溃/被杀/输出了 SDK 不识别的成功行）。必须终止循环，
                                    // 否则 ReadLine 持续返回 null 导致 100% CPU 空转 + 永久挂起（#1a）。
                                    if (stalled)
                                    {
                                        return AlyResponse.NG("download stalled: no output for " + stallTimeoutSeconds + "s, process killed");
                                    }
                                    return AlyResponse.NG("download interrupted (aly-client exited before finish line)");
                                }
                                lastLineAt = DateTime.UtcNow;
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
                                    try { progress?.Invoke(result.Data.File, result.Data.Index / (double)result.Data.Total); }
                                    catch { } // 进度回调异常不得打断下载（事件隔离）
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

        /// <summary>
        /// 回滚命令默认超时（秒）。回滚要做目录改名 + 关闭/等待主进程退出 +
        /// 崩溃恢复分支（可能多次 rename 重试），30 秒不够——超时被 Kill 会把
        /// 回滚留在中间态（由崩溃恢复机制兜底，但用户体验差）。
        /// </summary>
        public const int RollbackTimeoutSeconds = 600;

        /// <summary>回滚到指定版本（超时 600s，见 RollbackTimeoutSeconds）</summary>
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

            return RunAsync<RollbackData>(args, alyExePath, RollbackTimeoutSeconds);
        }

        // ══════════════════════════════════════════════════
        //  内部
        // ══════════════════════════════════════════════════

        /// <summary>短命令默认超时（秒）：check/list 等只读命令</summary>
        private const int DefaultRunTimeoutSeconds = 30;

        private static Task<AlyResponse<T>> RunAsync<T>(string arguments, string alyExePath)
            => RunAsync<T>(arguments, alyExePath, DefaultRunTimeoutSeconds);

        private static Task<AlyResponse<T>> RunAsync<T>(string arguments, string alyExePath, int timeoutSeconds)
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

                        // 旧实现先 WaitAll 再检查 HasExited：进程挂起时读流永远不结束，
                        // timeoutSeconds 形同虚设（self_update 卡住会阻塞 SDK 主循环启动）。
                        // 这里把超时放到读流上，超时即杀进程并返回失败。
                        if (!Task.WaitAll(new Task[] { stdoutTask, stderrTask }, timeoutSeconds * 1000))
                        {
                            try { process.Kill(); } catch { }
                            // 让读流任务在 dispose 之前收尾，避免后台任务访问已释放的流。
                            try { Task.WaitAll(new Task[] { stdoutTask, stderrTask }, 2000); } catch { }
                            return AlyResponse<T>.NG("aly-client.exe timed out after " + timeoutSeconds + "s and was killed");
                        }

                        var stdout = stdoutTask.Result;
                        var stderr = stderrTask.Result;

                        // 流已 EOF，进程通常已退出；短暂收尸，避免 ExitCode 访问未退出进程抛异常。
                        if (!process.WaitForExit(1000))
                        {
                            try { process.Kill(); } catch { }
                            return AlyResponse<T>.NG("aly-client.exe exited streams but process did not exit");
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
