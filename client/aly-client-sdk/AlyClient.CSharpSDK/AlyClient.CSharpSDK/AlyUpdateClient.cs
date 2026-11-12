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

        private readonly object _errorLock = new object();
        private bool _isError;
        private string _lastErrorMsg;

        /// <summary>后台主循环任务：Dispose 时等待其退出（最长 5 秒）</summary>
        private Task _mainLoopTask;

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

        /// <summary>
        /// 应用更新前需要关闭的本进程名（不带 .exe，多个用逗号分隔）。
        /// 宿主应用必须设置（如 Process.GetCurrentProcess().ProcessName）：
        /// 不设置时更新器在替换文件阶段探测到目录被本进程占用后会直接强杀，
        /// 应用没有机会优雅退出（停止监听、保存设置）。
        /// </summary>
        public string MustCloseProcessNames { get; set; }

        /// <summary>
        /// 空闲（无更新）时的轮询间隔（秒，#7）。默认 300 秒 + 0-60 秒随机抖动；
        /// 小于 60 时按 60 处理（兜底下限）。失败重试不受此值影响（保持 5 秒快速重试）。
        /// 旧版每秒轮询一次（每秒拉起一个 aly-client.exe 进程 + 1-2 次 HTTP），
        /// N 台客户端即 N req/s 纯轮询负载。
        /// </summary>
        public int IdlePollIntervalSeconds { get; set; }

        private static readonly Random IdleJitter = new Random();
        private static readonly object IdleJitterLock = new object();

        /// <summary>
        /// 计算本轮空闲睡眠时长（毫秒）：IdlePollIntervalSeconds（默认 300）
        /// + 0-60 秒抖动，避免客户端群同时打服务器。下限 60 秒。
        /// Random 非线程安全：抖动取值在专用锁内完成（多实例/多线程场景）。
        /// </summary>
        private int IdleSleepMs()
        {
            int seconds = IdlePollIntervalSeconds > 0 ? IdlePollIntervalSeconds : 300;
            if (seconds < 60) seconds = 60;
            lock (IdleJitterLock)
            {
                return (seconds + IdleJitter.Next(0, 60)) * 1000;
            }
        }

        /// <summary>
        /// 可中断睡眠（毫秒）：Cancel() 后最多 1 秒内退出，而非睡满全程。
        /// 旧版 Thread.Sleep：Cancel/Dispose 后循环最长再挂 6 分钟（300s+60s 抖动），
        /// 宿主关闭被后台线程拖住。
        /// </summary>
        private static void InterruptibleSleep(int milliseconds, CancellationToken token)
        {
            var remaining = milliseconds;
            while (remaining > 0 && !token.IsCancellationRequested)
            {
                var chunk = remaining > 1000 ? 1000 : remaining;
                Thread.Sleep(chunk);
                remaining -= chunk;
            }
        }

        /// <param name="updatorExePath">Defaults to ../UpdateFolder/aly-client.exe relative to BaseDirectory</param>
        public AlyUpdateClient(string updatorExePath = null)
        {
            UpdatorExePath = updatorExePath
                ?? Path.Combine(AppDomain.CurrentDomain.BaseDirectory, @"..\UpdateFolder\aly-client.exe");
            _cts = new CancellationTokenSource();
            IsRunning = true;

            // 启动任务前捕获 token：Dispose 会把 _cts 置空，任务在自更新之后
            // 再访问 _cts.Token 会 NullReference（自更新越久越容易撞上）。
            var token = _cts.Token;

            _mainLoopTask = Task.Factory.StartNew(() =>
            {
                // 自更新：正常路径统一委派 aly-client 的 self_update 命令；
                // 目标缺失/旧版更新器不认识该命令时由 UpdateSelf 内部回退到直接替换。
                try
                {
                    UpdateSelf();
                }
                catch (Exception ex)
                {
                    LogError("self-update failed: " + ex.Message);
                }
                MainLoop(token);
            });
        }

        /// <summary>Stop the background polling loop. Idempotent.</summary>
        public void Cancel()
        {
            try { _cts?.Cancel(); } catch (ObjectDisposedException) { }
        }

        /// <summary>
        /// Cancel and release resources. 等待主循环退出（最长 5 秒）：
        /// 旧版 Cancel 后立即 Dispose 掉 _cts，循环可能在下一次 token 检查前
        /// 拿到已释放的 token（ObjectDisposedException 吞在 catch 里，但循环
        /// 仍要睡满当前 IdleSleep 才退）。现在睡眠是可中断的，等待让"关闭即停"成立。
        /// </summary>
        public void Dispose()
        {
            Cancel();
            var task = _mainLoopTask;
            if (task != null)
            {
                try { task.Wait(5000); } catch { }
            }
            try { _cts?.Dispose(); } catch (ObjectDisposedException) { }
            _cts = null;
        }

        /// <summary>
        /// 自更新更新器：正常路径统一委派 aly-client 自己的 self_update 命令
        /// （与 UI / 裸控制台共用同一套 SHA 比较、先备后换、回滚与更新锁）。
        ///
        /// 仅在以下情况回退到直接替换（ReplaceUpdaterDirect）：
        ///  - 目标缺失：命令根本起不来（崩溃/被删）；
        ///  - 命令进程级失败且没有 .aly.lock：上游更新器还是旧版（不认识
        ///    self_update）或进程崩溃/超时。没有这个兜底，新 SDK 撞上旧更新器
        ///    会永久卡住升级链。
        /// 锁被占用（另一更新进行中）时不兜底，避免绕过 .aly.lock。
        /// </summary>
        private void UpdateSelf()
        {
            if (string.IsNullOrEmpty(UpdatorExePath))
            {
                LogError("self-update skipped: UpdatorExePath is empty");
                return;
            }
            string src = Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "aly-client.exe");

            // 目标缺失：命令无法启动，直接恢复。
            if (!File.Exists(UpdatorExePath))
            {
                ReplaceUpdaterDirect(src, UpdatorExePath);
                return;
            }

            var result = AlyApi.SelfUpdateAsync(UpdatorExePath).Result;
            if (result != null && result.IsSuccess)
            {
                var d = result.Data;
                if (d != null)
                {
                    LogError("self-update: updated=" + d.Updated + " reason=" + d.Reason +
                             (string.IsNullOrEmpty(d.OldPath) ? "" : " old=" + d.OldPath));
                }
                else
                {
                    LogError("self-update OK: " + UpdatorExePath);
                }
                return;
            }

            string msg = result != null && !string.IsNullOrEmpty(result.ErrorMsg)
                ? result.ErrorMsg : "unknown";
            string lockPath = Path.Combine(Path.GetDirectoryName(UpdatorExePath) ?? "", ".aly.lock");
            if (File.Exists(lockPath))
            {
                LogError("self-update skipped: another update in progress (" + msg + ")");
                return;
            }

            LogError("self_update command failed: " + msg + "; fallback to direct replace");
            ReplaceUpdaterDirect(src, UpdatorExePath);
        }

        /// <summary>
        /// 直接替换更新器（fallback）：与 `AlyUpdateUIClient` 的 UI 副本落位共用
        /// `UpdaterPayloadHelper` 的"临时文件 + SHA256 校验 + 原子替换"原语
        /// （重试 3 次、非 Windows 回退改名式事务），本类只负责日志口径。
        ///
        /// 只在 self_update 命令不可用或目标缺失时使用；正常路径不再走这里。
        ///
        /// 失败不静默（#8）：更新器损坏会破坏后续所有更新，必须留痕并保留
        /// 旧更新器（不清空目标）。
        /// </summary>
        private static void ReplaceUpdaterDirect(string src, string target)
        {
            var err = UpdaterPayloadHelper.ReplaceDirect(src, target);
            if (err == null)
            {
                LogError("self-update OK (direct fallback): " + target);
            }
            else
            {
                LogError("self-update fallback failed: " + (err.Message ?? "unknown"));
            }
        }

        /// <summary>写 SDK 诊断日志（失败静默）。</summary>
        private static void LogError(string msg)
        {
            SdkLogHelper.Write(msg);
        }

        /// <summary>Cancel and release resources. 见上（已前移至 Cancel 旁，等待主循环退出）。</summary>

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
                            InterruptibleSleep(5000, token);
                            continue;
                        }

                        ClearError();
                        if (!status.Data.HasUpdate)
                        {
                            // 无待处理更新：复位残留状态，避免宿主面板停在旧状态。
                            // 典型场景：已提示"发现新版本/正在下载"后网络中断（或本机处于
                            // downloaded 状态但服务器不可达，client 现在返回 has_update=false），
                            // 此时断网不应继续显示"可更新/正在下载"。仅在状态需要变化时
                            // 发一次事件（不新增事件、不改线程模型）。
                            // 文案用中性的"Update status cleared"：这里的 has_update=false 可能是
                            // "服务器不可达、无法核对"，不等价于核对过的"没有更新"。
                            if (TryClearStatus())
                            {
                                OnStatusChanged(AlyClientStatus.None, "Update status cleared");
                            }
                            // #7：空闲轮询间隔（默认 300s + 抖动）。
                            // 旧版每秒一次：每秒一个 aly-client.exe 进程 + 1-2 次 HTTP，
                            // XP 上进程创建成本高，且 N 客户端 = N req/s 打 DB。
                            InterruptibleSleep(IdleSleepMs(), token);
                            continue;
                        }

                        if (status.Data.NeedDownloadUpdate)
                        {
                            Status = AlyClientStatus.DiscoveredUpdate;
                            OnStatusChanged(Status, "Found new version: " + status.Data.NewVersion);

                            if (!status.Data.ForceUpdate)
                            {
                                // 事件隔离：宿主在确认回调里抛异常，不应让异常跳过本轮下载
                                // （否则进 catch 分支 5 秒重查、下载永远不开始）。
                                try { RequestDownloadUpdate?.Invoke(status.Data.NewVersion); }
                                catch (Exception ex) { LogError("RequestDownloadUpdate handler threw: " + ex.Message); }
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
                                    // "已是最新"= 无更新稳态：走空闲间隔（#7），不再 1 秒后立刻重查
                                    InterruptibleSleep(IdleSleepMs(), token);
                                    continue;
                                }
                                Thread.Sleep(1000);
                                continue;
                            }
                        }

                        Status = AlyClientStatus.DownloadedUpdate;
                        OnStatusChanged(Status, "Ready to apply Update");

                        if (!status.Data.ForceUpdate)
                        {
                            // 事件隔离（同 RequestDownloadUpdate）
                            try { RequestApplyUpdate?.Invoke(status.Data.NewVersion); }
                            catch (Exception ex) { LogError("RequestApplyUpdate handler threw: " + ex.Message); }
                        }

                        Status = AlyClientStatus.ApplyUpdate;
                        OnStatusChanged(Status, "Applying update...");

                        // apply spawn 失败上报：旧版丢弃返回值，更新器根本没拉起时
                        // 宿主只在 5 秒后看到"无更新"轮询，误以为更新完成了。
                        var applyResult = AlyApi.ApplyUpdateAsync(UpdatorExePath, MustCloseProcessNames);
                        if (applyResult != null && !applyResult.IsSuccess)
                        {
                            OnError("apply spawn failed: " + (applyResult.ErrorMsg ?? "unknown"));
                        }
                        // 不 break：apply_update 成功后宿主程序会被重启；
                        // 如果失败或用户取消则继续轮询后续更新。
                        // 30 秒（消 churn）：apply_update 是独立进程（RunAsyncAlone 不等待），
                        // 旧版 5 秒后继续 spawn check_update——锁会被 apply 拒绝，但每 5 秒
                        // 拉起一个进程白跑（XP 进程创建贵，日志也会刷屏）。
                        InterruptibleSleep(30000, token);
                    }
                    catch (Exception ex)
                    {
                        OnError("update exception: " + ex.Message);
                        InterruptibleSleep(5000, token);
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

        /// <summary>
        /// 把当前状态复位为 <see cref="AlyClientStatus.None"/>，返回是否真的发生了变化。
        /// 检查与置位在同一次 _statusLock 内完成（Status 对外宣称线程安全），状态事件由调用方在锁外发出。
        /// </summary>
        private bool TryClearStatus()
        {
            lock (_statusLock)
            {
                if (_status == AlyClientStatus.None) return false;
                _status = AlyClientStatus.None;
                return true;
            }
        }

        private void OnStatusChanged(AlyClientStatus s, string msg)
        {
            // 事件隔离：宿主事件处理器抛异常不得炸掉主循环（循环一挂更新就停了）。
            // Status 属性本身已先行设置，丢一次事件不影响状态正确性。
            var handler = StatusChanged;
            if (handler != null)
            {
                try { handler(s, msg); }
                catch (Exception ex) { LogError("StatusChanged handler threw: " + ex.Message); }
            }
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
            if (handler != null)
            {
                try { handler(msg); }
                catch (Exception ex) { LogError("ErrorStatusChanged handler threw: " + ex.Message); }
            }
        }

        private void ClearError()
        {
            Action<string> handler;
            lock (_errorLock)
            {
                if (!_isError) return;
                _isError = false;
                _lastErrorMsg = null;
                handler = ErrorStatusChanged;
            }
            // 事件在锁外触发（旧实现锁内触发，处理器重入/慢会拖住所有 OnError 调用方）
            if (handler != null)
            {
                try { handler(null); } // null = error cleared
                catch (Exception ex) { LogError("ErrorStatusChanged handler threw: " + ex.Message); }
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
