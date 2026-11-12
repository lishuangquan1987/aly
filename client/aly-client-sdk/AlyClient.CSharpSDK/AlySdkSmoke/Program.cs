using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Threading;
using AlyClient.CSharpSDK;
using AlyClient.CSharpSDK.Models;

namespace AlySdkSmoke
{
    /// <summary>
    /// SD 组 SDK 集成用例宿主驱动程序（net40，与 SDK 生产口径一致）。
    ///
    /// 用法：
    ///   AlySdkSmoke.exe all  <alyClientExePath>            依次执行 SD1-SD13
    ///   AlySdkSmoke.exe <id> <alyClientExePath>            执行单个用例（id: 1..16 或 sd1..sd16）
    ///
    /// alyClientExePath 指向测试环境的 pkg\UpdateFolder\aly-client.exe
    /// （e2e 脚本已完成：启动 server、创建项目、发布版本、构造客户端目录）。
    ///
    /// 输出协议（供 e2e 脚本逐行解析）：
    ///   PASS: SDx 名称
    ///   FAIL: SDx 名称（原因）
    ///   SUMMARY: PASS=n FAIL=n
    /// 退出码：0 全过 / 1 有失败。
    ///
    /// 注意：本程序本身充当"宿主应用"角色——部分用例（SD8 apply 链路）会触发
    /// aly-client.exe 关闭 must-close 进程名指定的进程，默认指向本进程名
    /// （fake host 名 = AlySdkSmoke），apply 期间进程会被关闭属预期行为，
    /// 由 e2e 脚本从外部重启并继续后续用例。
    /// </summary>
    internal static class Program
    {
        private static int _pass;
        private static int _fail;

        private static void Ok(string msg)
        {
            Console.WriteLine("PASS: " + msg);
            _pass++;
        }

        private static void Bad(string msg)
        {
            Console.WriteLine("FAIL: " + msg);
            _fail++;
        }

        private static void Assert(bool cond, string okMsg, string failMsg)
        {
            if (cond) Ok(okMsg); else Bad(failMsg);
        }

        private static string _exePath;

        private static int Main(string[] args)
        {
            // 输出协议要能被自动化逐行解析：net40 控制台默认用 OEM 代码页（中文机器为 936/GBK），
            // 被重定向到管道时 e2e 侧按 UTF-8 解码就会得到乱码。这里显式钉死 UTF-8。
            // 与断言逻辑无关，纯粹是"输出契约"—— 不加会导致所有中文 PASS 文案在 CI 里变乱码。
            try { Console.OutputEncoding = System.Text.Encoding.UTF8; }
            catch { /* 非交互/受限环境忽略：保持默认编码也能跑 */ }

            if (args.Length < 2)
            {
                Console.WriteLine("用法: AlySdkSmoke.exe <all|caseId> <alyClientExePath>");
                Console.WriteLine("  caseId: 1..13 或 sd1..sd13");
                return 2;
            }
            _exePath = Path.GetFullPath(args[1]);
            if (!File.Exists(_exePath))
            {
                Console.WriteLine("FAIL: aly-client.exe 不存在: " + _exePath);
                return 2;
            }

            string selector = args[0].Trim().ToLowerInvariant();
            var cases = new Dictionary<string, Action>
            {
                { "1", Sd1 }, { "sd1", Sd1 },
                { "2", Sd2 }, { "sd2", Sd2 },
                { "3", Sd3 }, { "sd3", Sd3 },
                { "4", Sd4 }, { "sd4", Sd4 },
                { "5", Sd5 }, { "sd5", Sd5 },
                { "6", Sd6 }, { "sd6", Sd6 },
                { "7", Sd7 }, { "sd7", Sd7 },
                { "8", Sd8 }, { "sd8", Sd8 },
                { "9", Sd9 }, { "sd9", Sd9 },
                { "10", Sd10 }, { "sd10", Sd10 },
                { "11", Sd11 }, { "sd11", Sd11 },
                { "12", Sd12 }, { "sd12", Sd12 },
                { "13", Sd13 }, { "sd13", Sd13 },
                // sd14 是 e2e 用例 101 的驱动入口：真正跑 AlyUpdateClient 的
                // 后台 MainLoop（自动 check → download → apply）。
                { "14", Sd14 }, { "sd14", Sd14 },
                // sd15 是用例 124 的驱动入口：Launch 拉 UI 的失败路径
                // （UI 缺失 / Dispose 后 / 连续失败），不需要真实窗口，可确定性验证。
                { "15", Sd15 }, { "sd15", Sd15 },
                // sd16 是用例 134 的驱动入口：AlyUpdateClient 构造函数委派 self_update
                { "16", Sd16 }, { "sd16", Sd16 },
                // sd17 是用例 137 的驱动入口：AlyUpdateUIClient 拉起前自动补齐两件套
                { "17", Sd17 }, { "sd17", Sd17 },
            };

            if (selector == "all")
            {
                // SD8 依赖外部重启宿主进程，单独跑（e2e 脚本编排）；
                // "all" 内跳过 SD8 以免自杀后中断序列，输出 SKIP 说明。
                // SD14 同理（它会走过 apply，可能关闭本进程），由 e2e 单独编排。
                for (int i = 1; i <= 14; i++)
                {
                    if (i == 8)
                    {
                        Console.WriteLine("SKIP: SD8 apply 全链路（需宿主进程重启编排，由 e2e 单独执行 sd8）");
                        continue;
                    }
                    if (i == 14)
                    {
                        Console.WriteLine("SKIP: SD14 MainLoop 全链路（会走 apply，需 e2e 编排）");
                        continue;
                    }
                    Console.WriteLine("=== SD" + i + " ===");
                    TryRun(cases[i.ToString()], i);
                }
            }
            else if (cases.ContainsKey(selector))
            {
                int id = int.Parse(selector.TrimStart('s', 'd'));
                Console.WriteLine("=== SD" + id + " ===");
                TryRun(cases[selector], id);
            }
            else
            {
                Console.WriteLine("FAIL: 未知用例号 " + args[0]);
                return 2;
            }

            Console.WriteLine("SUMMARY: PASS=" + _pass + " FAIL=" + _fail);
            return _fail == 0 ? 0 : 1;
        }

        private static void TryRun(Action action, int id)
        {
            try { action(); }
            catch (Exception ex)
            {
                Bad("SD" + id + " 未捕获异常: " + ex.Message);
            }
        }

        // ── SD 组用例实现 ─────────────────────────────────────────

        /// <summary>SD1 check_update 正常返回：isSuccess + has_update + current_version 字段完整</summary>
        private static void Sd1()
        {
            var r = AlyApi.CheckUpdateAsync(_exePath).Result;
            Assert(r.IsSuccess, "SD1 check_update isSuccess",
                "SD1 check_update 失败: " + (r.ErrorMsg ?? "?"));
            if (r.IsSuccess)
            {
                Assert(r.Data != null && r.Data.CurrentVersion != null, "SD1 data.current_version 字段存在",
                    "SD1 data 结构异常");
                Console.WriteLine("  info: has_update=" + r.Data.HasUpdate + " current=" + r.Data.CurrentVersion);
            }
        }

        /// <summary>SD2 EOF 语义：指向不存在的 exe → NG 而非挂死（10s 超时内返回）</summary>
        private static void Sd2()
        {
            string fake = Path.Combine(Path.GetTempPath(), "aly-smoke-" + Guid.NewGuid().ToString("N").Substring(0, 8) + ".exe");
            var sw = Stopwatch.StartNew();
            var r = AlyApi.CheckUpdateAsync(fake).Result;
            sw.Stop();
            Assert(!r.IsSuccess, "SD2 不存在的 exe 返回失败",
                "SD2 不存在的 exe 竟返回成功（数据异常）");
            Assert(sw.ElapsedMilliseconds < 10000, "SD2 快速失败不挂死（" + sw.ElapsedMilliseconds + "ms）",
                "SD2 耗时 " + sw.ElapsedMilliseconds + "ms（应 <10s）");
        }

        /// <summary>SD3 超时配置：RunAsync 短命令默认 30s 超时可配置性由代码常量保证（此处验证常量值）</summary>
        private static void Sd3()
        {
            Ok("SD3 RollbackTimeoutSeconds = " + AlyApi.RollbackTimeoutSeconds + " (期望 600)");
            Assert(AlyApi.RollbackTimeoutSeconds == 600, "SD3 回滚超时常量 600s", "SD3 回滚超时常量异常");
            Ok("SD3 DefaultDownloadStallTimeoutSeconds = " + AlyApi.DefaultDownloadStallTimeoutSeconds + " (期望 1800)");
            Assert(AlyApi.DefaultDownloadStallTimeoutSeconds == 1800, "SD3 下载停流看门狗常量 1800s",
                "SD3 下载停流看门狗常量异常");
        }

        /// <summary>SD4 进度回调：download_update 期间 progress(file, ratio) 被调用且 ratio 单调不减</summary>
        private static void Sd4()
        {
            var lastRatio = -1.0;
            var callCount = 0;
            var orderOk = true;
            var r = AlyApi.DownloadUpdateAsync(_exePath, (file, ratio) =>
            {
                callCount++;
                if (ratio < lastRatio - 0.0001) orderOk = false; // DONE 序号/总数应单调
                lastRatio = Math.Max(lastRatio, ratio);
            }).Result;
            Assert(r.IsSuccess || (r.ErrorMsg ?? "").Contains("already at latest version"),
                "SD4 download 成功或已是最新",
                "SD4 download 失败: " + (r.ErrorMsg ?? "?"));
            Assert(callCount > 0, "SD4 进度回调被调用（" + callCount + " 次）",
                "SD4 进度回调零调用（无 DONE 行或回调未接）");
            Assert(orderOk, "SD4 进度比率单调不减", "SD4 进度比率出现回退");
        }

        /// <summary>SD5 进度回调异常隔离：回调抛异常不影响下载完成</summary>
        private static void Sd5()
        {
            var r = AlyApi.DownloadUpdateAsync(_exePath, (file, ratio) =>
            {
                throw new InvalidOperationException("宿主回调故意抛异常（SD5）");
            }).Result;
            Assert(r.IsSuccess || (r.ErrorMsg ?? "").Contains("already at latest version"),
                "SD5 回调抛异常不影响下载",
                "SD5 回调异常传导至下载链路: " + (r.ErrorMsg ?? "?"));
        }

        /// <summary>SD6 check_update 断网语义（e2e 已停服务器时调用）：isSuccess:true + has_update:false</summary>
        private static void Sd6()
        {
            // 前置：e2e 脚本此刻已停服务器（由脚本编排，本用例只调用）
            var r = AlyApi.CheckUpdateAsync(_exePath).Result;
            Assert(r.IsSuccess, "SD6 断网 check isSuccess:true（不挂死不报错）",
                "SD6 断网 check 返回失败: " + (r.ErrorMsg ?? "?"));
            if (r.IsSuccess)
            {
                Assert(!r.Data.HasUpdate, "SD6 断网 has_update:false", "SD6 断网误报待应用更新");
            }
        }

        /// <summary>SD7 list_rollback_versions 正常返回列表结构</summary>
        private static void Sd7()
        {
            var r = AlyApi.ListRollbackVersionsAsync(_exePath).Result;
            Assert(r.IsSuccess, "SD7 list_rollback_versions isSuccess",
                "SD7 list_rollback_versions 失败: " + (r.ErrorMsg ?? "?"));
            if (r.IsSuccess && r.Data != null && r.Data.Versions != null)
            {
                Console.WriteLine("  info: 可回滚版本数 = " + r.Data.Versions.Length + "，当前 " + r.Data.CurrentVersion);
            }
        }

        /// <summary>
        /// SD8 apply 全链路（需 e2e 编排重启宿主）：本进程设为 must-close 目标，
        /// apply 后本进程被关闭；e2e 脚本重启后运行 sd9 验证 apply 结果。
        /// 此处只触发 apply（RunAsyncAlone 不等待，立即返回）。
        /// </summary>
        private static void Sd8()
        {
            string self = Process.GetCurrentProcess().ProcessName;
            var r = AlyApi.ApplyUpdateAsync(_exePath, self);
            Assert(r.IsSuccess, "SD8 apply spawn 成功",
                "SD8 apply spawn 失败: " + (r.ErrorMsg ?? "?"));
            // spawn 成功后 aly-client.exe 将在 closeTimeout 内关闭本进程——
            // 留 60s 窗口，等待被关闭（e2e 侧观察退出后重启）。
            Thread.Sleep(60000);
            // 走到这里说明 60s 内没被关闭（无更新可 apply 或 must-close 未生效）
            Ok("SD8 60s 内未被 apply 关闭（可能无待应用更新，e2e 编排判定结果）");
        }

        /// <summary>SD9 apply 后状态核验（e2e 重启宿主后调用）：version.json 的 version 已切换</summary>
        private static void Sd9()
        {
            // e2e 脚本重启宿主前会把期望版本写入环境变量 ALY_EXPECT_VERSION
            string expect = Environment.GetEnvironmentVariable("ALY_EXPECT_VERSION");
            if (string.IsNullOrEmpty(expect))
            {
                Console.WriteLine("SKIP: SD9 未设置 ALY_EXPECT_VERSION（e2e 编排专用）");
                return;
            }
            string versionJson = Path.Combine(Path.GetDirectoryName(_exePath), "version.json");
            string json = File.ReadAllText(versionJson);
            Assert(json.Contains("\"version\": \"" + expect + "\""),
                "SD9 apply 后 version=" + expect,
                "SD9 version.json 未指向期望版本 " + expect + "（内容: " + json + "）");
        }

        /// <summary>SD10 事件隔离：StatusChanged 处理器抛异常，MainLoop 不中断（IsRunning 保持 true）</summary>
        private static void Sd10()
        {
            using (var client = new AlyUpdateClient(_exePath)
            {
                MustCloseProcessNames = Process.GetCurrentProcess().ProcessName
            })
            {
                client.StatusChanged += (s, m) => { throw new InvalidOperationException("宿主状态处理器故意抛异常（SD10）"); };
                Thread.Sleep(5000); // 等至少一轮 check（首次无等待直接 check）
                Assert(client.IsRunning, "SD10 事件处理器抛异常后循环仍运行",
                    "SD10 事件异常炸掉了主循环（IsRunning=false）");
                client.Cancel();
            }
        }

        /// <summary>SD11 Cancel/Dispose 响应速度：Cancel 后 IsRunning 在 5s 内转 false</summary>
        private static void Sd11()
        {
            var client = new AlyUpdateClient(_exePath);
            Thread.Sleep(2000); // 循环启动
            var sw = Stopwatch.StartNew();
            client.Dispose();
            sw.Stop();
            Assert(!client.IsRunning, "SD11 Dispose 后 IsRunning=false",
                "SD11 Dispose 后循环未退出");
            Assert(sw.ElapsedMilliseconds < 6000, "SD11 Dispose 5s 内完成（" + sw.ElapsedMilliseconds + "ms）",
                "SD11 Dispose 耗时 " + sw.ElapsedMilliseconds + "ms（超时，检查 InterruptibleSleep）");
        }

        /// <summary>SD12 Dispose 幂等：连续 Dispose 两次不抛异常</summary>
        private static void Sd12()
        {
            var client = new AlyUpdateClient(_exePath);
            Thread.Sleep(1000);
            client.Dispose();
            try
            {
                client.Dispose();
                Ok("SD12 二次 Dispose 幂等");
            }
            catch (Exception ex)
            {
                Bad("SD12 二次 Dispose 抛异常: " + ex.Message);
            }
        }

        /// <summary>SD13 事件全套订阅（RequestDownload/RequestApply/Status/Error）不抛未捕获异常</summary>
        private static void Sd13()
        {
            using (var client = new AlyUpdateClient(_exePath))
            {
                client.StatusChanged += (s, m) => { };
                client.ErrorStatusChanged += m => { };
                client.RequestDownloadUpdate += v => { };
                client.RequestApplyUpdate += v => { };
                Thread.Sleep(5000);
                Assert(client.IsRunning, "SD13 全事件订阅运行正常",
                    "SD13 全事件订阅后循环异常退出");
                client.Cancel();
            }
        }

        /// <summary>
        /// SD14 MainLoop 全链路（e2e 用例 101 专用）：让 AlyUpdateClient 的**后台
        /// 主循环**自己完成 check → download → apply，而不是像 SD8 那样只 spawn
        /// 一次 apply_update。
        ///
        /// 为什么需要它：SD1..SD13 都是"直接调 AlyApi 的某个命令"，没有任何一条
        /// 覆盖 MainLoop 的编排逻辑（AlyUpdateClient.cs:237 的 while 循环）。
        /// 而 MainLoop 才是宿主真正跑的路径——下载、ForceUpdate 判定、
        /// "already at latest version" 降级、apply spawn 都在这里串起来。
        ///
        /// 观测方式不靠内部状态，只输出**事件与最终磁盘结果**，由 e2e 侧断言：
        ///  - `PASS: SD14 观察到 XxxStatus`（AlyClientStatus 的走向）
        ///  - `PASS: SD14 apply 已 spawn`
        ///  - 最终版本由 e2e 读 version.json 判定（本用例不碰文件系统）
        ///
        /// 注意：本进程就是 must-close 目标。走到 apply 时 aly-client.exe 会关闭
        /// 本进程——所以 e2e 侧必须允许"被关闭"，不能等退出码。
        /// </summary>
        private static void Sd14()
        {
            string self = Process.GetCurrentProcess().ProcessName;
            var seen = new System.Collections.Generic.List<string>();
            var seenLock = new object();
            var appliedSpawned = new ManualResetEventSlim(false);

            using (var client = new AlyUpdateClient(_exePath))
            {
                client.MustCloseProcessNames = self;
                // 空闲间隔拉到上限仍需 60s 兜底，这里把"发现更新"的等待窗口
                // 交给 e2e：只要首轮立刻 check，几秒内就会进 Downloading。
                client.StatusChanged += (s, msg) =>
                {
                    lock (seenLock) { seen.Add(s.ToString()); }
                };
                client.RequestApplyUpdate += v => { appliedSpawned.Set(); };

                // 首轮 check 是立刻执行的（构造函数里直接进 MainLoop，无空闲等待）。
                // 给下载 + apply spawn 留 90s：50MB 级文件在本机约 1~3s。
                var deadline = DateTime.UtcNow.AddSeconds(90);
                while (DateTime.UtcNow < deadline)
                {
                    string[] snapshot;
                    lock (seenLock) { snapshot = seen.ToArray(); }
                    bool reachedApply = false;
                    for (int i = 0; i < snapshot.Length; i++)
                    {
                        if (snapshot[i] == "ApplyUpdate") reachedApply = true;
                    }
                    if (reachedApply) break;
                    Thread.Sleep(200);
                }

                string[] final;
                lock (seenLock) { final = seen.ToArray(); }

                // 逐段断言：MainLoop 必须真的走过发现 → 下载（→ 应用）
                Assert(Array.IndexOf(final, "DiscoveredUpdate") >= 0,
                    "SD14 MainLoop 发现新版本（DiscoveredUpdate）",
                    "SD14 未观察到 DiscoveredUpdate，实际序列: " + string.Join(" → ", final));
                Assert(Array.IndexOf(final, "DownloadingUpdate") >= 0,
                    "SD14 MainLoop 自己发起下载（DownloadingUpdate）",
                    "SD14 未观察到 DownloadingUpdate（下载不是 MainLoop 干的？）实际: " + string.Join(" → ", final));
                Assert(Array.IndexOf(final, "DownloadedUpdate") >= 0,
                    "SD14 下载完成（DownloadedUpdate）",
                    "SD14 未观察到 DownloadedUpdate，实际: " + string.Join(" → ", final));

                // apply：ForceUpdate=true 时 SDK 不会触发 RequestApplyUpdate 事件
                //（看 MainLoop 里 `if (!status.Data.ForceUpdate)` 才 Invoke），
                // 所以这里不把事件当唯一判据，改为看状态是否推进到 ApplyUpdate。
                Assert(appliedSpawned.IsSet || Array.IndexOf(final, "ApplyUpdate") >= 0,
                    "SD14 MainLoop 进入 apply 阶段",
                    "SD14 未进入 apply 阶段，实际: " + string.Join(" → ", final));

                Console.WriteLine("  info: 状态序列 = " + string.Join(" → ", final));

                client.Cancel();
            }
        }

        /// <summary>
        /// SD15 Launch 拉 UI 的失败路径（用例 124 的驱动入口）。
        ///
        /// 用例 94 只测了"UI 存在 → Launch 返回 OK、退出码可读"的成功路径。
        /// UI 缺失（部署漏 candence / 杀软隔离 / 只装了 client 没装 UI）时，
        /// Launch 必须 **返回 NG 且不抛异常**——这是 SDK 调用方（宿主界面）
        /// 唯一能拿到错误信息的通道；若它抛异常穿透到调用方，宿主会直接崩溃。
        ///
        /// 三个必须成立的语义：
        ///   ① UI 路径不存在 → IsSuccess=false 且 ErrorMsg 非空（含 not found 提示）
        ///   ② 连续多次 Launch 失败 → 每次都返回 NG，不累积状态也不抛异常（幂等）
        ///   ③ 失败后再 Dispose → 不抛异常（失败路径不得留下未释放的 Process 句柄）
        /// </summary>
        private static void Sd15()
        {
            string missing = Path.Combine(
                Path.GetDirectoryName(_exePath), "definitely-no-such-ui-" + Guid.NewGuid().ToString("N") + ".exe");
            Assert(!File.Exists(missing), "SD15 前提：UI 路径确实不存在", "SD15 准备的路径意外存在: " + missing);

            var ui = new AlyUpdateUIClient(missing);
            bool started = false;
            ui.Started += _ => { started = true; };

            // ① 单次 Launch：必须 NG + 有 errorMsg，不是抛异常
            AlyResponse r1 = null;
            try
            {
                r1 = ui.Launch(AlyUpdateUiMode.Auto);
            }
            catch (Exception ex)
            {
                Bad("SD15 ① UI 缺失时 Launch 抛异常（应返回 NG）: " + ex.GetType().Name + ": " + ex.Message);
                return;
            }
            Assert(r1 != null && !r1.IsSuccess,
                "SD15 ① UI 缺失时 Launch 必须返回 NG",
                "SD15 ① UI 缺失却返回 OK（isSuccess=true），宿主会以为界面已打开");
            Assert(!string.IsNullOrEmpty(r1.ErrorMsg),
                "SD15 ① NG 必须带 errorMsg（调用方唯一的诊断来源）",
                "SD15 ① NG 但 errorMsg 为空，宿主只能显示空白错误");
            if (!string.IsNullOrEmpty(r1.ErrorMsg))
            {
                Console.WriteLine("  info: errorMsg = " + r1.ErrorMsg);
                bool mentions = r1.ErrorMsg.IndexOf("not found", StringComparison.OrdinalIgnoreCase) >= 0
                             || r1.ErrorMsg.IndexOf("aly-client-ui", StringComparison.OrdinalIgnoreCase) >= 0;
                Assert(mentions,
                    "SD15 ① errorMsg 应指明是 aly-client-ui.exe 找不到",
                    "SD15 ① errorMsg 未提及 ui 路径/not found: " + r1.ErrorMsg);
            }
            Assert(!started, "SD15 ① 失败路径不得触发 Started 事件", "SD15 ① 失败却触发了 Started");

            // ② 连续 3 次：每次都要 NG，且不抛
            for (int i = 1; i <= 3; i++)
            {
                try
                {
                    var r = ui.Launch(AlyUpdateUiMode.Check);
                    Assert(r != null && !r.IsSuccess,
                        "SD15 ② 第 " + i + " 次失败 Launch 仍应 NG",
                        "SD15 ② 第 " + i + " 次返回 OK（状态被前次失败污染？）");
                }
                catch (Exception ex)
                {
                    Bad("SD15 ② 第 " + i + " 次 Launch 抛异常: " + ex.GetType().Name + ": " + ex.Message);
                    return;
                }
            }

            // ③ 失败后 Dispose 不得抛（句柄/事件订阅要清理干净）
            try
            {
                ui.Dispose();
            }
            catch (Exception ex)
            {
                Bad("SD15 ③ 失败路径 Dispose 抛异常: " + ex.GetType().Name + ": " + ex.Message);
                return;
            }

            // ④ Dispose 后再 Launch：必须被 disposed 守卫挡住并返回 NG（不崩）
            try
            {
                var r4 = ui.Launch(AlyUpdateUiMode.Auto);
                Assert(r4 != null && !r4.IsSuccess,
                    "SD15 ④ Dispose 后 Launch 应返回 NG（disposed 守卫）",
                    "SD15 ④ Dispose 后 Launch 返回 OK");
            }
            catch (Exception ex)
            {
                Bad("SD15 ④ Dispose 后 Launch 抛异常: " + ex.GetType().Name + ": " + ex.Message);
            }
        }
        /// <summary>
        /// SD16 自更新委派（用例 134）：AlyUpdateClient 构造函数应调用 self_update
        /// 命令，把 UpdateFolder 的 aly-client.exe 换成主程序目录随版本分发的副本。
        ///
        /// 判定主要由 e2e 侧完成（比较主目录与 UpdateFolder 的 SHA256、检查 .old
        /// 判断走的是命令而非直接替换兜底）；smoke 在 ALY_EXPECT_UPDATER_SHA
        /// 存在时等待收敛，避免构造函数后台任务未完成就退出造成竞态。
        /// </summary>
        private static void Sd16()
        {
            string expected = Environment.GetEnvironmentVariable("ALY_EXPECT_UPDATER_SHA");
            using (var client = new AlyUpdateClient(_exePath))
            {
                if (!string.IsNullOrEmpty(expected))
                {
                    var deadline = DateTime.UtcNow.AddSeconds(15);
                    while (DateTime.UtcNow < deadline)
                    {
                        if (string.Equals(TryComputeSha256(_exePath), expected, StringComparison.OrdinalIgnoreCase))
                            break;
                        Thread.Sleep(200);
                    }
                }
                else
                {
                    Thread.Sleep(4000);
                }
                client.Cancel();
            }

            if (!string.IsNullOrEmpty(expected))
            {
                Assert(string.Equals(TryComputeSha256(_exePath), expected, StringComparison.OrdinalIgnoreCase),
                    "SD16 构造函数已把 UpdateFolder 的 aly-client.exe 换成主目录副本",
                    "SD16 self_update 未收敛：更新器 SHA 与主目录副本不一致");
            }
            else
            {
                Ok("SD16 AlyUpdateClient 构造完成（未提供 ALY_EXPECT_UPDATER_SHA，不做收敛断言）");
            }
        }

        /// <summary>
        /// SD17 拉起 UI 前自动补齐两件套（用例 137）：现场故障形态——发布包把
        /// aly-client / aly-client-ui 投放到主程序目录，但 UpdateFolder 里没有 UI
        /// 副本，宿主调 LaunchAuto 曾经只会拿到 "aly-client-ui.exe not found"。
        ///
        /// 断言（全部不依赖真的打开窗口；UI 副本用非可执行文件充当，启动失败属预期）：
        ///   ① 副本落位：Launch 之后 UpdateFolder/aly-client-ui.exe 存在，且 SHA256
        ///      与 ALY_EXPECT_UI_SHA（主程序目录同名副本）一致；
        ///   ② 两处都缺 → NG + errorMsg 含 not found/aly-client-ui，不抛异常、
        ///      不触发 Started；
        ///   ③ 单实例内只尝试一次：②之后在可读位置造一个同名"源副本"再 Launch，
        ///      副本仍不得出现（否则宿主定时重试会变成进程/IO 风暴）；
        ///   ④ 已一致：再 Launch 不得改写副本（SHA256 不变）。
        /// </summary>
        private static void Sd17()
        {
            string expected = Environment.GetEnvironmentVariable("ALY_EXPECT_UI_SHA");
            string uiDst = Path.Combine(Path.GetDirectoryName(_exePath), "aly-client-ui.exe");

            // ① 副本缺失 + 主程序目录有 → Launch 必须先把副本落位
            var ui = new AlyUpdateUIClient(uiDst, "zh-CN");
            bool started = false;
            ui.Started += _ => { started = true; };
            try
            {
                var r = ui.Launch(AlyUpdateUiMode.Auto);
                // UI 副本是"非可执行文件"充当 → 启动失败返回 NG 属预期；落位才是断言对象。
                Console.WriteLine("  info: sd17 ① launchIsSuccess=" + (r != null && r.IsSuccess)
                    + " errorMsg=" + (r == null ? "<null>" : r.ErrorMsg));
            }
            catch (Exception ex)
            {
                Bad("SD17 ① Launch 抛异常（补齐路径不得把异常穿透到宿主界面）: " + ex.GetType().Name + ": " + ex.Message);
                return;
            }

            Assert(File.Exists(uiDst),
                "SD17 ① UpdateFolder 缺 UI 副本时 Launch 必须自动从主程序目录落位",
                "SD17 ① Launch 之后 UI 副本仍不存在: " + uiDst);
            if (!string.IsNullOrEmpty(expected))
            {
                Assert(string.Equals(TryComputeSha256(uiDst), expected, StringComparison.OrdinalIgnoreCase),
                    "SD17 ① 落位的 UI 副本必须与主程序目录副本一致（SHA256）",
                    "SD17 ① 落位副本 SHA256 与主程序目录副本不一致");
            }

            // ② 两处都缺 → NG（不抛异常、不触发 Started）
            string missingDst = Path.Combine(Path.GetDirectoryName(_exePath), "sd17-missing-ui.exe");
            var ui2 = new AlyUpdateUIClient(missingDst, "zh-CN");
            bool started2 = false;
            ui2.Started += _ => { started2 = true; };
            AlyResponse r2 = null;
            try
            {
                r2 = ui2.Launch(AlyUpdateUiMode.Auto);
            }
            catch (Exception ex)
            {
                Bad("SD17 ② 两处都缺时 Launch 抛异常（应返回 NG）: " + ex.GetType().Name + ": " + ex.Message);
                return;
            }
            Assert(r2 != null && !r2.IsSuccess,
                "SD17 ② 两处都没有 UI 副本时 Launch 必须返回 NG",
                "SD17 ② 两处都缺却返回 OK（宿主会以为界面已打开）");
            Assert(!string.IsNullOrEmpty(r2.ErrorMsg) && (r2.ErrorMsg.IndexOf("not found", StringComparison.OrdinalIgnoreCase) >= 0
                    || r2.ErrorMsg.IndexOf("aly-client-ui", StringComparison.OrdinalIgnoreCase) >= 0),
                "SD17 ② NG 必须带可诊断的 errorMsg（含 not found / aly-client-ui）",
                "SD17 ② errorMsg 不可诊断: " + (r2 == null ? "<null>" : r2.ErrorMsg));
            Assert(!started2, "SD17 ② 失败路径不得触发 Started", "SD17 ② 失败却触发了 Started");

            // ③ 单实例内只尝试一次：在自身目录造一个同名的"源副本"后，副本仍不得
            //    出现（②已经尝试过；若还会重试，宿主定时器会把进程/IO 打满）。
            //    源用本机可读文件冒充（_exePath 是真实更新器 = 可读），与主程序目录无关。
            string srcForMissing = Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "sd17-missing-ui.exe");
            try { File.Delete(srcForMissing); } catch { }
            File.Copy(_exePath, srcForMissing, true);
            Assert(File.Exists(srcForMissing), "SD17 ③ 前置：同名源副本已就位", "SD17 ③ 造源副本失败: " + srcForMissing);
            ui2.Launch(AlyUpdateUiMode.Auto);
            Assert(!File.Exists(missingDst),
                "SD17 ③ 同一实例内不重复尝试补齐（失败后不再派进程）",
                "SD17 ③ 第二次 Launch 仍然尝试了补齐（副本被落位）: " + missingDst);
            try { File.Delete(srcForMissing); } catch { }

            // ④ 已一致：再 Launch 不得改写副本
            string shaBefore = TryComputeSha256(uiDst);
            try { ui.Launch(AlyUpdateUiMode.Auto); } catch { }
            Assert(!string.IsNullOrEmpty(shaBefore)
                    && string.Equals(TryComputeSha256(uiDst), shaBefore, StringComparison.OrdinalIgnoreCase),
                "SD17 ④ 副本已一致时 Launch 不得改写它",
                "SD17 ④ 一致路径改写了副本（SHA256 变化）");

            try { ui.Dispose(); } catch { }
            try { ui2.Dispose(); } catch { }
        }

        /// <summary>尽力计算文件 SHA256；文件正被替换时返回 null（调用方轮询重试）。</summary>
        private static string TryComputeSha256(string path)
        {
            try
            {
                using (var fs = new FileStream(path, FileMode.Open, FileAccess.Read, FileShare.ReadWrite))
                using (var sha = System.Security.Cryptography.SHA256.Create())
                {
                    byte[] hash = sha.ComputeHash(fs);
                    return BitConverter.ToString(hash).Replace("-", "").ToLowerInvariant();
                }
            }
            catch
            {
                return null;
            }
        }
    }
}
