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
            _cts = new CancellationTokenSource();
            IsRunning = true;

            var selfCheckUpdateResult = AlyApi.CheckSelfUpdateAsync(UpdatorExePath).Result;
            if (selfCheckUpdateResult.IsSuccess && selfCheckUpdateResult.Data.NeedUpdate)
            {
                try
                {
                    File.Copy(Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "aly-client.exe"),
                        UpdatorExePath, true);
                }
                catch (Exception) { }
            }

            Task.Factory.StartNew(() => MainLoop(_cts.Token));
        }

        /// <summary>Stop the background polling loop. Idempotent.</summary>
        public void Cancel()
        {
            try { _cts?.Cancel(); } catch (ObjectDisposedException) { }
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
                                RequestDownloadUpdate?.Invoke(status.Data.NewVersion);
                            }

                            Status = AlyClientStatus.DownloadingUpdate;
                            OnStatusChanged(Status, "Downloading...");

                            var downloadResult = AlyApi.DownloadUpdateAsync(UpdatorExePath, (fileName, progress) =>
                            {
                                OnStatusChanged(Status, string.Format("Downloading {0}... {1:F0}%", fileName, progress * 100));
                            }).Result;
                            if (!downloadResult.IsSuccess) { Thread.Sleep(1000); continue; }
                        }

                        Status = AlyClientStatus.DownloadedUpdate;
                        OnStatusChanged(Status, "Ready to apply Update");

                        if (!status.Data.ForceUpdate)
                        {
                            RequestApplyUpdate?.Invoke(status.Data.NewVersion);
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
            if (handler != null) handler(s, msg);
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
            if (handler != null) handler(msg);
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
