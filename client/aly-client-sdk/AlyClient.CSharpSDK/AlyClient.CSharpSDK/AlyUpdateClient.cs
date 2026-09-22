using System;
using System.IO;
using System.Threading;
using System.Threading.Tasks;

namespace AlyClient.CSharpSDK
{
    /// <summary>
    /// aly-client.exe CLI wrapper. Runs a background loop: check → download → apply.
    /// Call <see cref="Cancel"/> to stop polling, <see cref="Dispose"/> to release resources.
    /// </summary>
    public class AlyUpdateClient : IDisposable
    {
        private CancellationTokenSource _cts;
        private string _updateExePath;
        private readonly object _statusLock = new object();
        private AlyClientStatus _status;
        private readonly SynchronizationContext _syncContext;

        private readonly object _errorLock = new object();
        private bool _isError;
        private string _lastErrorMsg;

        /// <summary>Thread-safe current status</summary>
        public AlyClientStatus Status
        {
            get { lock (_statusLock) return _status; }
            private set { lock (_statusLock) _status = value; }
        }

        /// <summary>True while the background loop is running</summary>
        public bool IsRunning { get; private set; }

        /// <summary>True if the last check_update or download failed</summary>
        public bool IsError
        {
            get { lock (_errorLock) return _isError; }
        }

        /// <summary>aly-client.exe absolute path</summary>
        public string UpdatorExePath
        {
            get => _updateExePath;
            set => _updateExePath = string.IsNullOrEmpty(value) ? value : Path.GetFullPath(value);
        }

        /// <param name="updatorExePath">Defaults to ../UpdateFolder/aly-client.exe relative to BaseDirectory</param>
        public AlyUpdateClient(string updatorExePath = null)
        {
            UpdatorExePath = updatorExePath
                ?? Path.Combine(AppDomain.CurrentDomain.BaseDirectory, @"..\UpdateFolder\aly-client.exe");
            // 捕获创建线程的同步上下文（WPF/UI 线程创建实例时为 UI 上下文），
            // 用于把后台线程的事件回调封送到 UI 线程（#21）。
            _syncContext = SynchronizationContext.Current;
            _cts = new CancellationTokenSource();
            IsRunning = true;

            Task.Factory.StartNew(() =>
            {
                var selfCheckUpdateResult = AlyApi.CheckSelfUpdateAsync(UpdatorExePath).Result;
                if (selfCheckUpdateResult.IsSuccess && selfCheckUpdateResult.Data.NeedUpdate)
                {
                    UpdateSelf();
                }
                MainLoop(_cts.Token);
            });
        }

        /// <summary>
        /// 将 action 异步封送到创建实例时的同步上下文（UI 线程）执行。
        /// 适用于状态通知类回调（StatusChanged / ErrorStatusChanged）：
        /// 宿主在 UI 线程创建实例时自动回到 UI 线程（#21）；
        /// 非 UI 线程创建实例（_syncContext == null）时保持原行为。
        /// </summary>
        private void Raise(Action action)
        {
            if (_syncContext != null)
            {
                _syncContext.Post(_ => action(), null);
            }
            else
            {
                action();
            }
        }

        /// <summary>
        /// 将 action **同步**封送到同步上下文执行（Send 阻塞后台线程直到宿主处理完）。
        /// 仅用于确认类事件（RequestDownloadUpdate / RequestApplyUpdate）：宿主 handler
        /// 需要在 SDK 继续推进前同步完成（例如调用 Cancel() 否决下载/应用），
        /// 若用异步 Post，后台循环会立即继续并把状态推到 Downloading/Apply，
        /// 宿主的否决将迟到失效（#21 审查发现）。MainLoop 在后台线程，阻塞安全。
        /// </summary>
        private void RaiseSync(Action action)
        {
            if (_syncContext != null)
            {
                _syncContext.Send(_ => action(), null);
            }
            else
            {
                action();
            }
        }

        /// <summary>Stop the background polling loop. Idempotent.</summary>
        public void Cancel()
        {
            try { _cts?.Cancel(); } catch (ObjectDisposedException) { }
        }

        /// <summary>
        /// 自更新更新器：临时文件 + SHA256 校验 + 原子替换（File.Replace / File.Move），
        /// 目标被占用时指数退避重试（1s、2s、4s、8s、16s，共 5 次，约 31s）。
        /// 替换前先等待仍在运行的更新器进程退出（#22）。
        /// 失败不静默（#8）：更新器损坏会破坏后续所有更新，必须留痕并保留旧更新器（不清空目标）。
        /// </summary>
        private void UpdateSelf()
        {
            string src = Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "aly-client.exe");
            string tmp = UpdatorExePath + ".new";
            Exception lastErr = null;

            // #22：若上一次 apply 拉起的更新器进程尚未退出，File.Replace 会因目标被占用失败。
            // 先等待 aly-client.exe 进程退出（最多 10s，步长 1s），再开始替换。
            for (int i = 0; i < 10 && IsProcessRunning("aly-client"); i++)
            {
                Thread.Sleep(1000);
            }

            int[] backoffs = { 1000, 2000, 4000, 8000, 16000 }; // 指数退避（毫秒）
            for (int i = 0; i < backoffs.Length; i++)
            {
                try
                {
                    File.Copy(src, tmp, true);
                    // 校验副本与源一致（防止复制截断）。源的完整性由主更新链路保证：
                    // 随版本下发的 aly-client.exe 在 download_update 时已按服务端
                    // MD5+SHA256 校验通过（server/get_all_files 提供可信哈希）。
                    if (ComputeSha256(tmp) != ComputeSha256(src))
                    {
                        throw new IOException("self-update checksum mismatch");
                    }
                    if (File.Exists(UpdatorExePath))
                    {
                        // 目标存在：原子替换（要求目标未被占用）
                        File.Replace(tmp, UpdatorExePath, null);
                    }
                    else
                    {
                        // 首次部署：目标不存在，直接改名
                        File.Move(tmp, UpdatorExePath);
                    }
                    LogError("self-update OK: " + UpdatorExePath);
                    return;
                }
                catch (Exception ex)
                {
                    lastErr = ex;
                    try { File.Delete(tmp); } catch { }
                    // 非 Windows 平台（netstandard2.0/Mono 等）File.Replace 抛
                    // PlatformNotSupportedException：回退为 删除目标 + 改名。
                    var pns = ex as PlatformNotSupportedException;
                    if (pns != null)
                    {
                        try
                        {
                            if (File.Exists(UpdatorExePath)) File.Delete(UpdatorExePath);
                            File.Move(tmp, UpdatorExePath);
                            LogError("self-update OK (move fallback): " + UpdatorExePath);
                            return;
                        }
                        catch (Exception ex2)
                        {
                            lastErr = ex2;
                        }
                    }
                    // 目标被占用（如上一更新进程尚未完全退出）：指数退避后重试（#22）。
                    // 最后一次失败不再 sleep，立即上报错误，避免无谓延迟（审查发现）。
                    if (i < backoffs.Length - 1)
                    {
                        Thread.Sleep(backoffs[i]);
                    }
                }
            }
            string msg = "self-update failed: " + (lastErr != null ? lastErr.Message : "unknown");
            LogError(msg);
        }

        /// <summary>判断指定进程名（不含 .exe）是否仍在运行（排除当前进程自身）。</summary>
        private static bool IsProcessRunning(string processName)
        {
            try
            {
                int selfId = System.Diagnostics.Process.GetCurrentProcess().Id;
                foreach (var p in System.Diagnostics.Process.GetProcessesByName(processName))
                {
                    if (p.Id != selfId)
                    {
                        return true;
                    }
                }
                return false;
            }
            catch
            {
                return false; // 无法枚举时保守返回 false，不阻塞自更新
            }
        }

        private static string ComputeSha256(string path)
        {
            using (var fs = new FileStream(path, FileMode.Open, FileAccess.Read))
            using (var sha = System.Security.Cryptography.SHA256.Create())
            {
                byte[] hash = sha.ComputeHash(fs);
                return BitConverter.ToString(hash).Replace("-", "").ToLowerInvariant();
            }
        }

        private static void LogError(string msg)
        {
            try
            {
                string log = Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "aly-client-sdk.log");
                File.AppendAllText(log, DateTime.Now.ToString("yyyy-MM-dd HH:mm:ss") + " " + msg + Environment.NewLine);
            }
            catch { }
        }

        /// <summary>Cancel and release resources.</summary>
        public void Dispose()
        {
            Cancel();
            try { _cts?.Dispose(); } catch (ObjectDisposedException) { }
            _cts = null;
        }

        private void MainLoop(CancellationToken token)
        {
            try
            {
                while (!token.IsCancellationRequested)
                {
                    try
                    {
                        var status = AlyApi.CheckUpdateAsync(UpdatorExePath).Result;
                        if (!status.IsSuccess)
                        {
                            OnError(status.ErrorMsg ?? "check_update failed");
                            Thread.Sleep(5000);
                            continue;
                        }

                        ClearError();
                        if (!status.Data.HasUpdate) { Thread.Sleep(1000); continue; }

                        if (status.Data.NeedDownloadUpdate)
                        {
                            Status = AlyClientStatus.DiscoveredUpdate;
                            OnStatusChanged(Status, "Found new version: " + status.Data.NewVersion);

                            if (!status.Data.ForceUpdate)
                            {
                                RaiseRequestDownload(status.Data.NewVersion);
                            }

                            Status = AlyClientStatus.DownloadingUpdate;
                            OnStatusChanged(Status, "Downloading...");

                            var downloadResult = AlyApi.DownloadUpdateAsync(UpdatorExePath, (fileName, progress) =>
                            {
                                // StatusChanged 携带文件名与百分比（0-100）
                                OnStatusChanged(Status, string.Format("Downloading {0}... {1}%", fileName, (int)Math.Round(progress * 100)));
                            }).Result;
                            if (!downloadResult.IsSuccess)
                            {
                                // #6 防御：client 已是最新版本时 download_update 返回
                                // "already at latest version"（文案与
                                // client/aly-client/cmd/download_update.go 保持一致），
                                // 应视为无需更新而非错误，避免强制更新场景下死循环报错。
                                if (downloadResult.ErrorMsg != null &&
                                    downloadResult.ErrorMsg.Contains("already at latest version"))
                                {
                                    ClearError();
                                    // 状态复位，避免宿主 UI 停留在 "Downloading..."（#6 审查发现）
                                    Status = AlyClientStatus.None;
                                    OnStatusChanged(Status, "No update needed");
                                }
                                Thread.Sleep(1000);
                                continue;
                            }
                        }

                        Status = AlyClientStatus.DownloadedUpdate;
                        OnStatusChanged(Status, "Ready to apply Update");

                        if (!status.Data.ForceUpdate)
                        {
                            RaiseRequestApply(status.Data.NewVersion);
                        }

                        Status = AlyClientStatus.ApplyUpdate;
                        OnStatusChanged(Status, "Applying update...");

                        AlyApi.ApplyUpdateAsync(UpdatorExePath);
                        // 不 break：apply_update 成功后宿主程序会被重启；
                        // 如果失败或用户取消则继续轮询后续更新。
                        Thread.Sleep(5000);
                    }
                    catch (Exception ex)
                    {
                        OnError("update exception: " + ex.Message);
                        Thread.Sleep(5000);
                    }
                }
            }
            catch (Exception ex)
            {
                // 顶层兜底，确保 IsRunning 正确
                try { OnError("MainLoop fatal: " + ex.Message); } catch { }
            }
            finally
            {
                IsRunning = false;
            }
        }

        private void OnStatusChanged(AlyClientStatus s, string msg)
        {
            var handler = StatusChanged;
            if (handler != null) Raise(() => handler(s, msg));
        }

        private void OnError(string msg)
        {
            lock (_errorLock)
            {
                if (!_isError)
                {
                    _isError = true;
                    _lastErrorMsg = msg;
                }
                else if (_lastErrorMsg == msg)
                {
                    return; // same error, skip
                }
                else
                {
                    _lastErrorMsg = msg;
                }
            }
            var handler = ErrorStatusChanged;
            if (handler != null) Raise(() => handler(msg));
        }

        /// <summary>通知宿主"需要确认下载"，同步封送到 UI 线程以保留否决窗口（#21）。</summary>
        private void RaiseRequestDownload(string version)
        {
            var handler = RequestDownloadUpdate;
            if (handler != null) RaiseSync(() => handler(version));
        }

        /// <summary>通知宿主"需要确认应用"，同步封送到 UI 线程以保留否决窗口（#21）。</summary>
        private void RaiseRequestApply(string version)
        {
            var handler = RequestApplyUpdate;
            if (handler != null) RaiseSync(() => handler(version));
        }

        private void ClearError()
        {
            lock (_errorLock)
            {
                if (_isError)
                {
                    _isError = false;
                    _lastErrorMsg = null;
                    var handler = ErrorStatusChanged;
                    if (handler != null) handler(null); // null = error cleared
                }
            }
        }

        /// <summary>Raised when download confirmation is needed (non-force updates)</summary>
        public event Action<string> RequestDownloadUpdate;

        /// <summary>Raised when apply confirmation is needed (non-force updates)</summary>
        public event Action<string> RequestApplyUpdate;

        /// <summary>Raised for every status change with a human-readable message</summary>
        public event Action<AlyClientStatus, string> StatusChanged;

        /// <summary>
        /// Raised when error state changes. msg = error text (new error or changed),
        /// msg = null (error recovered). Not raised when same error repeats.
        /// </summary>
        public event Action<string> ErrorStatusChanged;
    }
}
