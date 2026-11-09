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

            _mainLoopTask = Task.Factory.StartNew(() =>
            {
                // 宿主事件处理器异常不得炸掉循环（事件隔离）：
                // CheckSelfUpdateAsync 内部已捕获自身异常，这里再兜一层。
                try
                {
                    var selfCheckUpdateResult = AlyApi.CheckSelfUpdateAsync(UpdatorExePath).Result;
                    if (selfCheckUpdateResult.IsSuccess && selfCheckUpdateResult.Data.NeedUpdate)
                    {
                        UpdateSelf();
                    }
                }
                catch (Exception ex)
                {
                    LogError("self check-update failed: " + ex.Message);
                }
                MainLoop(_cts.Token);
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
        /// 自更新更新器：临时文件 + SHA256 校验 + 原子替换（File.Replace / File.Move），
        /// 目标被占用时重试 3 次（间隔 1s）。失败不静默（#8）：
        /// 更新器损坏会破坏后续所有更新，必须留痕并保留旧更新器（不清空目标）。
        /// </summary>
        private void UpdateSelf()
        {
            string src = Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "aly-client.exe");
            string tmp = UpdatorExePath + ".new";
            Exception lastErr = null;
            for (int i = 0; i < 3; i++)
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
                    // 非 Windows 平台（netstandard2.0/Mono 等）File.Replace 抛
                    // PlatformNotSupportedException。此时 tmp 仍在，走事务式回退
                    // MoveWithRollback（备份改名 → 落位 → 失败回滚）。
                    //
                    // 2026-11-09 修复两个缺陷：
                    //   ① 旧实现在判断异常类型之前就 File.Delete(tmp)，导致回退
                    //      分支里的 File.Move(tmp, ...) 必然失败——回退路径从未
                    //      真正可用过；
                    //   ② 旧回退是"删除目标 + 改名"，存在目标消失窗口：一旦删除
                    //      后改名失败（磁盘满/占用），更新器就彻底没了，而更新器
                    //      损坏会破坏后续所有更新（#8）。改为任何时刻目标要么是
                    //      旧版、要么是新版。
                    var pns = ex as PlatformNotSupportedException;
                    if (pns != null)
                    {
                        try
                        {
                            MoveWithRollback(tmp, UpdatorExePath);
                            LogError("self-update OK (move fallback): " + UpdatorExePath);
                            return;
                        }
                        catch (Exception ex2)
                        {
                            lastErr = ex2;
                            try { File.Delete(tmp); } catch { }
                            // 目标被占用等瞬时故障：等待后重试
                            Thread.Sleep(1000);
                            continue;
                        }
                    }
                    // Windows 正常路径失败（如上一更新进程尚未完全退出占用目标）：
                    // 清理 tmp 后等待重试
                    try { File.Delete(tmp); } catch { }
                    Thread.Sleep(1000);
                }
            }
            string msg = "self-update failed: " + (lastErr != null ? lastErr.Message : "unknown");
            LogError(msg);
        }

        /// <summary>
        /// 事务式改名落位（非 Windows 平台的 File.Replace 回退）：
        /// 目标改名到 .bak（原子） → tmp 改名到目标（原子） → 失败则把 .bak 改回目标。
        /// 与 Windows 的 File.Replace（原子替换、失败保留目标）语义对齐。
        /// .bak 残留与"目标缺失 + .bak 存在"的异常现场，均由下一次调用开头恢复。
        /// </summary>
        private static void MoveWithRollback(string tmp, string target)
        {
            string bak = target + ".bak";
            // 异常现场恢复：目标缺失但 .bak 存在（上次"备份成功、落位失败且回滚
            // 也失败"遗留的现场）——先把 .bak 还原为目标，再走正常替换流程。
            if (!File.Exists(target) && File.Exists(bak))
            {
                File.Move(bak, target);
            }
            bool movedToBak = false;
            if (File.Exists(target))
            {
                if (File.Exists(bak)) File.Delete(bak);
                File.Move(target, bak);
                movedToBak = true;
            }
            try
            {
                File.Move(tmp, target);
            }
            catch
            {
                // 回滚：把 .bak 改回目标，保证目标始终存在
                if (movedToBak && !File.Exists(target))
                {
                    try { File.Move(bak, target); } catch { }
                }
                throw;
            }
            if (movedToBak)
            {
                File.Delete(bak);
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
