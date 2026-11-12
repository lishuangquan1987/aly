using System;
using System.IO;

namespace AlyClient.CSharpSDK
{
    /// <summary>
    /// 文件指纹（SHA256）计算：更新器自更新与图形前端副本补齐的共用比较原语。
    ///
    /// 为什么要抽出来：`AlyUpdateClient`（更新器自更新）与 `AlyUpdateUIClient`
    /// （拉起 UI 前保证两件套就位）都要"比两份文件是否同内容"，且都要容忍
    /// "文件正被替换/被杀软占用"这类瞬时读失败——读失败返回 null，由调用方
    /// 按"不确定"处理，而不是抛异常穿透到宿主界面。
    /// </summary>
    internal static class Sha256Helper
    {
        /// <summary>
        /// 返回文件内容的 SHA256 十六进制串（小写）；读取失败（文件缺失、
        /// 正被独占、权限不足）时返回 null。
        /// </summary>
        public static string Of(string path)
        {
            if (string.IsNullOrEmpty(path))
            {
                return null;
            }
            try
            {
                // ReadWrite 共享：目标可能正被另一个实例替换，只读共享会被拒。
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

        /// <summary>
        /// 两份文件是否同内容。任一侧读不到（null）都返回 false —— 调用方按
        /// "不确定"走保守分支（补齐/委派），而不是误判为一致。
        /// </summary>
        public static bool Same(string a, string b)
        {
            string ha = Of(a);
            if (ha == null)
            {
                return false;
            }
            string hb = Of(b);
            return hb != null && ha == hb;
        }

        /// <summary>文件名（最后一个 \ 或 / 之后的部分）；空路径返回空串。</summary>
        public static string FileNameOf(string path)
        {
            if (string.IsNullOrEmpty(path))
            {
                return "";
            }
            int cut = -1;
            for (int i = 0; i < path.Length; i++)
            {
                if (path[i] == '\\' || path[i] == '/')
                {
                    cut = i;
                }
            }
            return cut < 0 ? path : path.Substring(cut + 1);
        }
    }
}
