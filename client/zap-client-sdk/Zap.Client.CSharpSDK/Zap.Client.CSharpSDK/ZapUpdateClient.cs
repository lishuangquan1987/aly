using System;
using System.IO;
using System.Threading;
using System.Threading.Tasks;

namespace Zap.Client.CSharpSDK
{
    /// <summary>
    /// zap-client.exe CLI wrapper. Calls zap-client.exe via Process,
    /// parses stdout JSON to drive the update lifecycle.
    /// </summary>
    public class ZapUpdateClient
    {
        public ZapClientStatus Status { get; private set; }

        private string _updateExePath;

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
            Task.Factory.StartNew(MainLoop);
        }

        private void MainLoop()
        {
            while (true)
            {
                Thread.Sleep(1000);

                var status = ZapApi.CheckUpdateAsync(UpdatorExePath).Result;
                if (!status.IsSuccess) continue;
                if (!status.Data.HasUpdate) continue;

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
                    if (!downloadResult.IsSuccess) continue;
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
        }

        private void OnStatusChanged(ZapClientStatus s, string msg)
        {
            StatusChanged?.Invoke(s, msg);
        }

        /// <summary>Raised when download confirmation is needed (non-force updates)</summary>
        public event Action<string> RequestDownloadUpdate;

        /// <summary>Raised when apply confirmation is needed (non-force updates)</summary>
        public event Action<string> RequestApplyUpdate;

        /// <summary>Raised for every status change with a human-readable message</summary>
        public event Action<ZapClientStatus, string> StatusChanged;
    }
}
