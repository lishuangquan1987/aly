using System;
using System.IO;
using System.Runtime.InteropServices;
using AlyClient.CSharpSDK;
using AlyClient.CSharpSDK.Models;

namespace AlyClient.Com
{
    /// <summary>
    /// VB6 / 易语言可用的 COM 外观。全部方法同步；返回值 bool，
    /// 详细状态通过只读属性读取。
    /// </summary>
    [ComVisible(true)]
    [Guid("c1a4e6b2-7f3d-4c8e-9a51-6d2b8f0e4c77")]
    [ProgId("AlyClient.Com.AlyComClient")]
    [ClassInterface(ClassInterfaceType.AutoDual)]
    public sealed class AlyComClient
    {
        public AlyComClient()
        {
            UpdaterPath = Path.Combine(
                AppDomain.CurrentDomain.BaseDirectory,
                @"..\UpdateFolder\aly-client.exe");
            Reset();
        }

        /// <summary>aly-client.exe 路径；默认 ../UpdateFolder/aly-client.exe。</summary>
        public string UpdaterPath { get; set; }

        public bool IsSuccess { get; private set; }
        public string ErrorMsg { get; private set; }

        public bool HasUpdate { get; private set; }
        public bool NeedDownloadUpdate { get; private set; }
        public bool ForceUpdate { get; private set; }
        public string CurrentVersion { get; private set; }
        public string NewVersion { get; private set; }
        public string ReleaseNotes { get; private set; }

        public bool NeedSelfUpdate { get; private set; }
        public bool Updated { get; private set; }
        public string SelfUpdateReason { get; private set; }
        public string OldPath { get; private set; }

        public string ProgressFile { get; private set; }
        public double ProgressRatio { get; private set; }

        public string RollbackVersions { get; private set; }
        public string RolledBackVersion { get; private set; }

        /// <summary>check_update。返回 IsSuccess；数据在属性中。</summary>
        public bool CheckUpdate()
        {
            Reset();
            try
            {
                var response = AlyApi.CheckUpdateAsync(UpdaterPath).Result;
                IsSuccess = response.IsSuccess;
                ErrorMsg = response.ErrorMsg ?? string.Empty;
                if (!response.IsSuccess || response.Data == null)
                {
                    return IsSuccess;
                }
                HasUpdate = response.Data.HasUpdate;
                NeedDownloadUpdate = response.Data.NeedDownloadUpdate;
                ForceUpdate = response.Data.ForceUpdate;
                CurrentVersion = response.Data.CurrentVersion ?? string.Empty;
                NewVersion = response.Data.NewVersion ?? string.Empty;
                ReleaseNotes = Join(response.Data.ReleaseNotes);
                return true;
            }
            catch (Exception ex)
            {
                return Fail(ex);
            }
        }

        /// <summary>check_self_update。结果在 NeedSelfUpdate。</summary>
        public bool CheckSelfUpdate()
        {
            Reset();
            try
            {
                var response = AlyApi.CheckSelfUpdateAsync(UpdaterPath).Result;
                IsSuccess = response.IsSuccess;
                ErrorMsg = response.ErrorMsg ?? string.Empty;
                if (response.IsSuccess && response.Data != null)
                {
                    NeedSelfUpdate = response.Data.NeedUpdate;
                }
                return IsSuccess;
            }
            catch (Exception ex)
            {
                return Fail(ex);
            }
        }

        /// <summary>self_update。结果在 Updated / OldPath / SelfUpdateReason。</summary>
        public bool SelfUpdate()
        {
            Reset();
            try
            {
                var response = AlyApi.SelfUpdateAsync(UpdaterPath).Result;
                IsSuccess = response.IsSuccess;
                ErrorMsg = response.ErrorMsg ?? string.Empty;
                if (response.IsSuccess && response.Data != null)
                {
                    Updated = response.Data.Updated;
                    SelfUpdateReason = response.Data.Reason ?? string.Empty;
                    OldPath = response.Data.OldPath ?? string.Empty;
                }
                return IsSuccess;
            }
            catch (Exception ex)
            {
                return Fail(ex);
            }
        }

        /// <summary>download_update，阻塞直到完成。进度在 ProgressFile / ProgressRatio。</summary>
        public bool DownloadUpdate()
        {
            Reset();
            try
            {
                var response = AlyApi.DownloadUpdateAsync(
                    UpdaterPath,
                    (file, ratio) =>
                    {
                        ProgressFile = file ?? string.Empty;
                        ProgressRatio = ratio;
                    }).Result;
                IsSuccess = response.IsSuccess;
                ErrorMsg = response.ErrorMsg ?? string.Empty;
                return IsSuccess;
            }
            catch (Exception ex)
            {
                return Fail(ex);
            }
        }

        /// <summary>apply_update：拉起独立更新进程后立即返回。</summary>
        public bool ApplyUpdate(string mustCloseProcessNames, int closeTimeoutSeconds)
        {
            Reset();
            try
            {
                var response = AlyApi.ApplyUpdateAsync(
                    UpdaterPath,
                    mustCloseProcessNames,
                    closeTimeoutSeconds <= 0 ? 30 : closeTimeoutSeconds);
                IsSuccess = response.IsSuccess;
                ErrorMsg = response.ErrorMsg ?? string.Empty;
                return IsSuccess;
            }
            catch (Exception ex)
            {
                return Fail(ex);
            }
        }

        /// <summary>list_rollback_versions。逗号分隔结果在 RollbackVersions。</summary>
        public bool ListRollbackVersions()
        {
            Reset();
            try
            {
                var response = AlyApi.ListRollbackVersionsAsync(UpdaterPath).Result;
                IsSuccess = response.IsSuccess;
                ErrorMsg = response.ErrorMsg ?? string.Empty;
                if (response.IsSuccess && response.Data != null)
                {
                    RollbackVersions = Join(response.Data.Versions);
                }
                return IsSuccess;
            }
            catch (Exception ex)
            {
                return Fail(ex);
            }
        }

        /// <summary>rollback。实际回滚版本在 RolledBackVersion。</summary>
        public bool Rollback(string version, string mustCloseProcessNames, int closeTimeoutSeconds)
        {
            Reset();
            try
            {
                var response = AlyApi.RollbackAsync(
                    UpdaterPath,
                    version,
                    mustCloseProcessNames,
                    closeTimeoutSeconds <= 0 ? 30 : closeTimeoutSeconds).Result;
                IsSuccess = response.IsSuccess;
                ErrorMsg = response.ErrorMsg ?? string.Empty;
                if (response.IsSuccess && response.Data != null)
                {
                    RolledBackVersion = response.Data.Version ?? string.Empty;
                }
                return IsSuccess;
            }
            catch (Exception ex)
            {
                return Fail(ex);
            }
        }

        private void Reset()
        {
            IsSuccess = false;
            ErrorMsg = string.Empty;
            HasUpdate = false;
            NeedDownloadUpdate = false;
            ForceUpdate = false;
            CurrentVersion = string.Empty;
            NewVersion = string.Empty;
            ReleaseNotes = string.Empty;
            NeedSelfUpdate = false;
            Updated = false;
            SelfUpdateReason = string.Empty;
            OldPath = string.Empty;
            ProgressFile = string.Empty;
            ProgressRatio = 0.0;
            RollbackVersions = string.Empty;
            RolledBackVersion = string.Empty;
        }

        private bool Fail(Exception ex)
        {
            IsSuccess = false;
            ErrorMsg = ex.Message;
            return false;
        }

        private static string Join(string[] values)
        {
            return values == null || values.Length == 0
                ? string.Empty
                : string.Join("\n", values);
        }
    }
}