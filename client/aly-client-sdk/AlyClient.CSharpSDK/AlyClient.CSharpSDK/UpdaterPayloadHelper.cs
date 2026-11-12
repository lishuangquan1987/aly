using System;
using System.IO;
using System.Threading;

namespace AlyClient.CSharpSDK
{
    /// <summary>
    /// 更新器两件套的共享落位原语：临时文件 + SHA256 校验 + 原子替换。
    ///
    /// 为什么要共享：`AlyUpdateClient`（更新器自更新兜底）与 `AlyUpdateUIClient`
    /// （拉起 UI 前补齐两件套）需要同一套"复制一份、校验、原子落位、失败保留
    /// 旧文件"的语义；两份实现会漂移，而这是"更新器自身还能不能继续工作"的底线路径。
    ///
    /// 正常路径不走这里：统一委派 aly-client 的 `self_update`（含 --include-ui）。
    /// 本原语只在命令不可用（旧版更新器/目标缺失/进程级失败）时兜底。
    ///
    /// 失败时**不**静默：调用方负责写日志并保留旧文件（不清空目标）——更新器
    /// 损坏会破坏后续所有更新。
    /// </summary>
    internal static class UpdaterPayloadHelper
    {
        /// <summary>落位重试次数与间隔（Windows 上常见原因是上一更新进程尚未完全退出）。</summary>
        public const int ReplaceRetries = 3;
        public const int ReplaceRetrySleepMs = 1000;

        /// <summary>
        /// 用 src 替换 target（临时文件 + SHA256 校验 + 原子替换）。
        ///
        /// 非 Windows 平台（netstandard2.0/Mono 等）`File.Replace` 抛
        /// PlatformNotSupportedException：回退到事务式 MoveWithRollback
        /// （备份改名 → 落位 → 失败回滚），任何时刻目标要么旧版要么新版。
        /// 校验不过或复制失败时仅重试（1s 间隔），目标保持原样。
        /// </summary>
        /// <returns>成功返回 null；失败返回最后一次异常。</returns>
        public static Exception ReplaceDirect(string src, string target)
        {
            if (!File.Exists(src))
            {
                return new IOException("source missing: " + src);
            }

            string tmp = target + ".new";
            Exception lastErr = null;
            for (int i = 0; i < ReplaceRetries; i++)
            {
                try
                {
                    File.Copy(src, tmp, true);
                    // 校验副本与源一致（防止复制截断）。源的完整性由主更新链路保证：
                    // 随版本下发的两件套在 download_update 时已按服务端 MD5+SHA256 校验。
                    if (!Sha256Helper.Same(tmp, src))
                    {
                        throw new IOException("checksum mismatch");
                    }

                    if (File.Exists(target))
                    {
                        // 目标存在：原子替换（要求目标未被占用）
                        File.Replace(tmp, target, null);
                    }
                    else
                    {
                        // 目标缺失（首次部署/崩溃现场）：直接改名
                        File.Move(tmp, target);
                    }
                    return null;
                }
                catch (Exception ex)
                {
                    lastErr = ex;
                    var pns = ex as PlatformNotSupportedException;
                    if (pns != null)
                    {
                        try
                        {
                            MoveWithRollback(tmp, target);
                            return null;
                        }
                        catch (Exception ex2)
                        {
                            lastErr = ex2;
                            try { File.Delete(tmp); } catch { }
                            Thread.Sleep(ReplaceRetrySleepMs);
                            continue;
                        }
                    }
                    // Windows 正常路径失败：清理 tmp 后等待重试
                    try { File.Delete(tmp); } catch { }
                    Thread.Sleep(ReplaceRetrySleepMs);
                }
            }
            return lastErr != null ? lastErr : new IOException("replace failed: " + target);
        }

        /// <summary>
        /// 事务式改名落位（非 Windows 平台的 File.Replace 回退）：
        /// 目标改名到 .bak（原子）→ tmp 改名到目标（原子）→ 失败则把 .bak 改回目标。
        /// 与 Windows 的 File.Replace（原子替换、失败保留目标）语义对齐。
        /// .bak 残留与"目标缺失 + .bak 存在"的异常现场，均由下一次调用开头恢复。
        /// </summary>
        public static void MoveWithRollback(string tmp, string target)
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
    }
}
