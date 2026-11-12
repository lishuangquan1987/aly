using System;
using System.IO;
using System.Text;
using AlyClient.CSharpSDK;
using AlyClient.CSharpSDK.Models;

namespace AlySdkConformance
{
    /// <summary>
    /// 用 tools/fake-aly-client 回放 spec/fixtures/scenarios 的协议级一致性用例。
    /// 不需要服务端、不需要真实 aly-client.exe，任何 CI 都能跑。
    ///
    /// 用法：AlySdkConformance.exe &lt;fake-aly-client.exe&gt; &lt;scenariosDir&gt;
    /// </summary>
    internal static class Program
    {
        private static int _pass;
        private static int _fail;
        private static string _fake;
        private static string _scenarioDir;

        private static int Main(string[] args)
        {
            if (args.Length < 2)
            {
                Console.WriteLine("用法: AlySdkConformance.exe <fake-aly-client.exe> <scenariosDir>");
                return 2;
            }
            _fake = Path.GetFullPath(args[0]);
            _scenarioDir = Path.GetFullPath(args[1]);
            try { Console.OutputEncoding = Encoding.UTF8; }
            catch { /* 受限环境忽略 */ }

            if (!File.Exists(_fake))
            {
                Console.Error.WriteLine("假客户端不存在: " + _fake);
                return 2;
            }
            if (!Directory.Exists(_scenarioDir))
            {
                Console.Error.WriteLine("场景目录不存在: " + _scenarioDir);
                return 2;
            }

            Case("check_update: 发现新版本字段完整", CheckAvailableUpdate);
            Case("check_update: 已是最新", CheckAlreadyLatest);
            Case("check_update: 服务器不可达", CheckOffline);
            Case("download_update: 进度回调与完成行", DownloadWithProgress);
            Case("download_update: 部分失败透传", DownloadPartialFailure);
            Case("download_update: 停流看门狗", DownloadStall);
            Case("apply_update: 拉起即返回", ApplyReturnsImmediately);
            Case("self_update: 检测与替换", SelfUpdateFlow);
            Case("self_update: --include-ui 的 ui_* 可选字段", SelfUpdateIncludeUiFlow);
            Case("rollback: 列表与回滚", RollbackFlow);
            Case("缺失更新器: 返回失败而不是抛异常", MissingExecutable);

            Console.WriteLine(string.Format("SUMMARY: PASS={0} FAIL={1}", _pass, _fail));
            return _fail == 0 ? 0 : 1;
        }

        private static string CheckAvailableUpdate()
        {
            UseScenario("update_available");
            AlyResponse<CheckUpdateData> r = AlyApi.CheckUpdateAsync(_fake).Result;
            if (!r.IsSuccess) return "NG: " + r.ErrorMsg;
            if (!r.Data.HasUpdate) return "has_update=false";
            if (!r.Data.NeedDownloadUpdate) return "need_download_update=false";
            if (r.Data.NewVersion != "1.0.1") return "new_version=" + r.Data.NewVersion;
            if (r.Data.ForceUpdate) return "force_update=true（期望 false）";
            if (r.Data.ReleaseNotes == null || r.Data.ReleaseNotes.Length != 2)
                return "release_notes 数量不符";
            return null;
        }

        private static string CheckAlreadyLatest()
        {
            UseScenario("already_latest");
            AlyResponse<CheckUpdateData> r = AlyApi.CheckUpdateAsync(_fake).Result;
            if (!r.IsSuccess) return "NG: " + r.ErrorMsg;
            if (r.Data.HasUpdate) return "has_update=true（期望 false）";
            return null;
        }

        private static string CheckOffline()
        {
            UseScenario("offline");
            AlyResponse<CheckUpdateData> r = AlyApi.CheckUpdateAsync(_fake).Result;
            if (!r.IsSuccess) return "NG: " + r.ErrorMsg;
            if (r.Data.HasUpdate) return "离线应为 has_update=false";
            return null;
        }

        private static string DownloadWithProgress()
        {
            UseScenario("update_available");
            string lastFile = null;
            double lastRatio = -1;
            AlyResponse r = AlyApi.DownloadUpdateAsync(_fake, (file, ratio) =>
            {
                lastFile = file;
                lastRatio = ratio;
            }).Result;
            if (!r.IsSuccess) return "NG: " + r.ErrorMsg;
            if (lastFile != "app.exe") return "进度文件=" + lastFile;
            if (lastRatio != 1d) return "进度比例=" + lastRatio;
            return null;
        }

        private static string DownloadPartialFailure()
        {
            UseScenario("download_partial_failure");
            AlyResponse r = AlyApi.DownloadUpdateAsync(_fake, null).Result;
            if (r.IsSuccess) return "部分失败应为 NG";
            if (r.ErrorMsg == null || !r.ErrorMsg.Contains("checksum mismatch"))
                return "errorMsg=" + r.ErrorMsg;
            return null;
        }

        private static string DownloadStall()
        {
            UseScenario("download_stall");
            // C# SDK 的看门狗每 5s 检查一次；这里把停止阈值设成 1s，约 5s 后杀进程。
            AlyResponse r = AlyApi.DownloadUpdateAsync(_fake, null, 1).Result;
            if (r.IsSuccess) return "停流应返回 NG";
            if (r.ErrorMsg == null || !r.ErrorMsg.Contains("stalled"))
                return "errorMsg=" + r.ErrorMsg;
            return null;
        }

        private static string ApplyReturnsImmediately()
        {
            UseScenario("update_available");
            DateTime started = DateTime.UtcNow;
            AlyResponse r = AlyApi.ApplyUpdateAsync(_fake, "AlySdkConformance", 30);
            TimeSpan elapsed = DateTime.UtcNow - started;
            if (!r.IsSuccess) return "NG: " + r.ErrorMsg;
            if (elapsed.TotalSeconds >= 3) return "apply 不应等待，耗时 " + elapsed.TotalSeconds + "s";
            return null;
        }

        private static string SelfUpdateFlow()
        {
            UseScenario("self_update");
            AlyResponse<CheckSelfUpdateData> check = AlyApi.CheckSelfUpdateAsync(_fake).Result;
            if (!check.IsSuccess) return "check_self_update NG: " + check.ErrorMsg;
            if (!check.Data.NeedUpdate) return "need_update=false（期望 true）";
            AlyResponse<SelfUpdateData> updated = AlyApi.SelfUpdateAsync(_fake).Result;
            if (!updated.IsSuccess) return "self_update NG: " + updated.ErrorMsg;
            if (!updated.Data.Updated) return "updated=false（期望 true）";
            if (updated.Data.OldPath != "aly-client.exe.old") return "old_path=" + updated.Data.OldPath;
            return null;
        }

        /// <summary>
        /// self_update --include-ui：ui_* 三个可选字段（协议 1.1.0 MINOR 新增）必须
        /// 能被反序列化，且不破坏既有字段口径。假客户端按命令名回放场景，旗标被忽略，
        /// 因此这里同时覆盖了"带旗标的调用能把命令行拼对"（拼错会让假客户端找不到命令
        /// 而退出码非 0 → NG）。
        /// </summary>
        private static string SelfUpdateIncludeUiFlow()
        {
            UseScenario("self_update_include_ui");
            AlyResponse<SelfUpdateData> r = AlyApi.SelfUpdateAsync(_fake, true).Result;
            if (!r.IsSuccess) return "NG: " + r.ErrorMsg;
            if (r.Data == null) return "data=null";
            if (r.Data.Updated) return "updated=true（场景为已是最新）";
            if (r.Data.Reason != "already_up_to_date") return "reason=" + r.Data.Reason;
            if (!r.Data.UiUpdated) return "ui_updated=false（期望 true）";
            if (r.Data.UiReason != "replaced") return "ui_reason=" + r.Data.UiReason;
            if (!string.IsNullOrEmpty(r.Data.UiOldPath)) return "ui_old_path=" + r.Data.UiOldPath;
            return null;
        }

        private static string RollbackFlow()
        {
            UseScenario("rollback");
            AlyResponse<ListRollbackData> list = AlyApi.ListRollbackVersionsAsync(_fake).Result;
            if (!list.IsSuccess) return "list NG: " + list.ErrorMsg;
            if (list.Data.Versions == null || list.Data.Versions.Length != 2)
                return "versions 数量不符";
            AlyResponse<RollbackData> rolled = AlyApi.RollbackAsync(_fake, "1.0.0").Result;
            if (!rolled.IsSuccess) return "rollback NG: " + rolled.ErrorMsg;
            if (rolled.Data.Version != "1.0.0") return "version=" + rolled.Data.Version;
            return null;
        }

        private static string MissingExecutable()
        {
            string missing = Path.Combine(Path.GetTempPath(), "aly-conformance-missing", "aly-client.exe");
            AlyResponse<CheckUpdateData> r = AlyApi.CheckUpdateAsync(missing).Result;
            if (r.IsSuccess) return "缺失更新器应为 NG";
            if (string.IsNullOrEmpty(r.ErrorMsg)) return "errorMsg 为空";
            return null;
        }

        private static void UseScenario(string name)
        {
            Environment.SetEnvironmentVariable("ALY_FAKE_SCENARIO", Path.Combine(_scenarioDir, name + ".json"));
        }

        private static void Case(string name, Func<string> body)
        {
            string error;
            try
            {
                error = body();
            }
            catch (Exception ex)
            {
                error = ex.ToString();
            }
            if (error == null)
            {
                _pass++;
                Console.WriteLine("PASS: " + name);
            }
            else
            {
                _fail++;
                Console.WriteLine("FAIL: " + name + " —— " + error);
            }
        }
    }
}