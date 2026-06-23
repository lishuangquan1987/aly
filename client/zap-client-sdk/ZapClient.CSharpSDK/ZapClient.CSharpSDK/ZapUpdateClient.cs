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

        /// <summary>Thread-safe current status</summary>
        public ZapClientStatus Status
        {
            get { lock (_statusLock) return _status; }
            private set { lock (_statusLock) _status = value; }
        }

        /// <summary>True while the background loop is running</summary>
        public bool IsRunning { get; private set; }

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
                try
                {
                    token.ThrowIfCancellationRequested();
                }
                catch (OperationCanceledException)
                {
                    break;
                }

                var status = ZapApi.CheckUpdateAsync(UpdatorExePath).Result;
                if (!status.IsSuccess) { Thread.Sleep(1000); continue; }
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
                        OnStatusChanged(Status, "Downloading... " + ((int)(progress * 100)) + "%");
                    }).Result;
                    if (!downloadResult.IsSuccess) { Thread.Sleep(1000); continue; }
                }

                Status = ZapClientStatus.DownloadedUpdate;
                OnStatusChanged(Status, "Update downloaded, ready to apply");

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

        /// <summary>Raised when download confirmation is needed (non-force updates)</summary>
        public event Action<string> RequestDownloadUpdate;

        /// <summary>Raised when apply confirmation is needed (non-force updates)</summary>
        public event Action<string> RequestApplyUpdate;

        /// <summary>Raised for every status change with a human-readable message</summary>
        public event Action<ZapClientStatus, string> StatusChanged;
    }
}
