using System;
using System.IO;
using System.Threading;
using System.Threading.Tasks;

namespace ZapClient.CSharpSDK
{
    /// <summary>
    /// zap-client.exe CLI wrapper. Runs a background loop: check → download → apply.
    /// Call <see cref="Cancel"/> to stop polling, <see cref="Dispose"/> to release resources.
    /// </summary>
    public class ZapUpdateClient : IDisposable
    {
        private CancellationTokenSource _cts;
        private string _updateExePath;
        private readonly object _statusLock = new object();
        private ZapClientStatus _status;

        private readonly object _errorLock = new object();
        private bool _isError;
        private string _lastErrorMsg;

        /// <summary>Thread-safe current status</summary>
        public ZapClientStatus Status
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

        /// <summary>zap-client.exe absolute path</summary>
        public string UpdatorExePath
        {
            get => _updateExePath;
            set => _updateExePath = string.IsNullOrEmpty(value) ? value : Path.GetFullPath(value);
        }

        /// <param name="updatorExePath">Defaults to ../UpdateFolder/zap-client.exe relative to BaseDirectory</param>
        public ZapUpdateClient(string updatorExePath = null)
        {
            UpdatorExePath = updatorExePath
                ?? Path.Combine(AppDomain.CurrentDomain.BaseDirectory, @"..\UpdateFolder\zap-client.exe");
            _cts = new CancellationTokenSource();
            IsRunning = true;

            var selfCheckUpdateResult = ZapApi.CheckSelfUpdateAsync(UpdatorExePath).Result;
            if (selfCheckUpdateResult.IsSuccess && selfCheckUpdateResult.Data.NeedUpdate)
            {
                try
                {
                    File.Copy(Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "zap-client.exe"),
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
            while (!token.IsCancellationRequested)
            {
                var status = ZapApi.CheckUpdateAsync(UpdatorExePath).Result;
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
                    Status = ZapClientStatus.DiscoveredUpdate;
                    OnStatusChanged(Status, "Found new version: " + status.Data.NewVersion);

                    if (!status.Data.ForceUpdate)
                    {
                        RequestDownloadUpdate?.Invoke(status.Data.NewVersion);
                    }

                    Status = ZapClientStatus.DownloadingUpdate;
                    OnStatusChanged(Status, "Downloading...");

                    var downloadResult = ZapApi.DownloadUpdateAsync(UpdatorExePath, (fileName, progress) =>
                    {
                        OnStatusChanged(Status, string.Format("Downloading {0}... {1:F0}%", fileName, progress * 100));
                    }).Result;
                    if (!downloadResult.IsSuccess) { Thread.Sleep(1000); continue; }
                }

                Status = ZapClientStatus.DownloadedUpdate;
                OnStatusChanged(Status, "Ready to apply Update");

                if (!status.Data.ForceUpdate)
                {
                    RequestApplyUpdate?.Invoke(status.Data.NewVersion);
                }

                Status = ZapClientStatus.ApplyUpdate;
                OnStatusChanged(Status, "Applying update...");

                ZapApi.ApplyUpdateAsync(UpdatorExePath);
                break;
            }

            IsRunning = false;
        }

        private void OnStatusChanged(ZapClientStatus s, string msg)
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
        public event Action<ZapClientStatus, string> StatusChanged;

        /// <summary>
        /// Raised when error state changes. msg = error text (new error or changed),
        /// msg = null (error recovered). Not raised when same error repeats.
        /// </summary>
        public event Action<string> ErrorStatusChanged;
    }
}
