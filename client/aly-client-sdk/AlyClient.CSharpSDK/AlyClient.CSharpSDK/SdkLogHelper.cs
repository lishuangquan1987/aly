using System;
using System.IO;

namespace AlyClient.CSharpSDK
{
    /// <summary>
    /// SDK 侧诊断日志（<c>&lt;宿主基目录&gt;/aly-client-sdk.log</c>）的共享写入原语。
    ///
    /// 为什么要共享：自更新委派、两件套补齐、事件处理器异常都往同一个文件写行，
    /// 一旦两处实现漂移（时间戳格式、换行、编码），排现场问题时要读两种格式。
    /// 写失败必须静默：日志是诊断线索，不是业务前置条件。
    /// </summary>
    internal static class SdkLogHelper
    {
        /// <summary>文件名（位于宿主基目录下）。</summary>
        public const string LogFileName = "aly-client-sdk.log";

        /// <summary>追加一行（失败静默）。</summary>
        public static void Write(string msg)
        {
            try
            {
                string log = Path.Combine(AppDomain.CurrentDomain.BaseDirectory, LogFileName);
                File.AppendAllText(log, DateTime.Now.ToString("yyyy-MM-dd HH:mm:ss") + " " + msg + Environment.NewLine);
            }
            catch { }
        }
    }
}
