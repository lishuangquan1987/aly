using System;
using System.Diagnostics;
using System.IO;
using AlyClient.CSharpSDK.Models;

namespace AlyClient.CSharpSDK
{
    /// <summary>
    /// aly-client-ui.exe 的启动模式，对应命令行参数 -mode。
    /// </summary>
    public enum AlyUpdateUiMode
    {
        /// <summary>
        /// 启动时自动检查：窗口默认隐藏，只有发现更新才显示；无更新或服务器不可达时静默退出。
        /// 推荐宿主在应用启动时调用。
        /// </summary>
        Auto,

        /// <summary>
        /// 交互式检查：用户主动点击「检查更新」时使用。会先显示「正在检查」，
        /// 无更新/出错也会提示并自动关闭，避免「点了没反应」。
        /// </summary>
        Check,

        /// <summary>直接进入更新流程（下载 + 应用），不再额外询问。</summary>
        Update,

        /// <summary>显示可回滚版本列表，由用户在窗口内选择回滚。</summary>
        Rollback,

        /// <summary>
        /// 静默检查：与 <see cref="Auto"/> 行为一致——无更新/出错时不打扰用户，
        /// 确认有更新后才显示窗口。
        /// </summary>
        Silent,
    }

    /// <summary>
    /// aly-client-ui.exe 封装：把「检查 → 下载 → 应用 → 重启主程序」整条更新链路
    /// 交给 aly-client-ui 的图形窗口完成，宿主只负责按场景拉起对应模式。
    ///
    /// 与 <see cref="AlyUpdateClient"/> 的区别：
    /// <list type="bullet">
    /// <item>旧类：宿主自建后台轮询 + 事件 + 阻塞等待用户确认，需要在应用内自绘更新 UI；</item>
    /// <item>本类：更新交互全部在 aly-client-ui 的独立窗口内完成，宿主不感知下载/应用细节，
    /// 也不需要自己处理「关闭主进程后重启」。</item>
    /// </list>
    ///
    /// 部署要求：aly-client-ui.exe 与 aly-client.exe 同目录，默认
    /// <c>&lt;应用目录&gt;\..\UpdateFolder\</c>；由 AlyClient.CSharpSDK 的 NuGet targets 自动下载部署
    /// （设置 <c>AlyIncludeClientUi=false</c> 可跳过 UI 下载）。
    ///
    /// <para>
    /// 说明：aly-client-ui 是单实例程序，重复拉起时后启动的进程会立即以退出码 0 结束，
    /// 因此 <see cref="Launch"/> 在上一次进程未退出时直接返回失败，避免宿主误以为已弹出窗口。
    /// </para>
    /// </summary>
    public class AlyUpdateUIClient : IDisposable
    {
        /// <summary>默认界面语言（zh-CN）。</summary>
        public const string DefaultLanguage = "zh-CN";

        private readonly object _sync = new object();
        private Process _process;
        private int? _lastExitCode;
        private bool _disposed;

        private string _uiExePath;

        /// <summary>aly-client-ui.exe 的绝对路径。赋值时自动转绝对路径。</summary>
        public string UiExePath
        {
            get { return _uiExePath; }
            set { _uiExePath = string.IsNullOrEmpty(value) ? value : Path.GetFullPath(value); }
        }

        /// <summary>界面语言：zh-CN / en-US。</summary>
        public string Language { get; set; }

        /// <summary>上一次拉起的进程是否仍在运行。</summary>
        public bool IsRunning
        {
            get
            {
                lock (_sync)
                {
                    return _process != null && !SafeHasExited(_process);
                }
            }
        }

        /// <summary>上一次运行的退出码；本次运行尚未结束时为 null。</summary>
        public int? LastExitCode
        {
            get { lock (_sync) return _lastExitCode; }
        }

        /// <summary>进程成功拉起后触发，参数为 PID。</summary>
        public event Action<int> Started;

        /// <summary>进程退出后触发，参数为退出码。</summary>
        public event Action<int> Exited;

        /// <param name="uiExePath">
        /// aly-client-ui.exe 路径；为空时取
        /// <c>&lt;应用目录&gt;\..\UpdateFolder\aly-client-ui.exe</c>。
        /// </param>
        /// <param name="language">界面语言 zh-CN / en-US，默认 zh-CN。</param>
        public AlyUpdateUIClient(string uiExePath = null, string language = DefaultLanguage)
        {
            UiExePath = string.IsNullOrEmpty(uiExePath)
                ? Path.GetFullPath(Path.Combine(AppDomain.CurrentDomain.BaseDirectory, @"..\UpdateFolder\aly-client-ui.exe"))
                : Path.GetFullPath(uiExePath);
            Language = string.IsNullOrWhiteSpace(language) ? DefaultLanguage : language;
        }

        /// <summary>按指定模式拉起 aly-client-ui。非阻塞，立即返回。</summary>
        public AlyResponse Launch(AlyUpdateUiMode mode)
        {
            if (_disposed)
                return AlyResponse.NG("AlyUpdateUIClient has been disposed.");

            if (!File.Exists(UiExePath))
                return AlyResponse.NG("aly-client-ui.exe not found: " + UiExePath
                    + "（请确认 UpdateFolder 已部署；构建时设置 AlyIncludeClientUi=true 自动下载）");

            lock (_sync)
            {
                // 上一次实例还在跑：单实例保护会让新进程直接退出，这里显式报错，
                // 避免调用方误以为已经弹出了窗口。
                if (_process != null && !SafeHasExited(_process))
                    return AlyResponse.NG("aly-client-ui is already running.");

                // 上一次实例已退出：先释放旧句柄再启动，避免每次检查泄漏一个 Process
                DisposeProcess(_process);
                _process = null;

                try
                {
                    var psi = new ProcessStartInfo
                    {
                        FileName = UiExePath,
                        Arguments = string.Format("-mode {0} -lang {1}", ToArgument(mode), Language),
                        UseShellExecute = false,
                        CreateNoWindow = true,
                        WorkingDirectory = Path.GetDirectoryName(UiExePath),
                    };

                    var process = new Process { StartInfo = psi };
                    process.EnableRaisingEvents = true;
                    process.Exited += OnProcessExited;

                    if (!process.Start())
                    {
                        // 启动失败：释放刚创建的 Process，否则句柄随失败次数累积
                        DisposeProcess(process);
                        return AlyResponse.NG("failed to start aly-client-ui.exe");
                    }

                    _process = process;
                    _lastExitCode = null;
                    var pid = process.Id;
                    RaiseStarted(pid);
                    return AlyResponse.OK();
                }
                catch (Exception ex)
                {
                    return AlyResponse.NG(ex);
                }
            }
        }

        /// <summary>启动时自动检查（有更新才弹窗）。</summary>
        public AlyResponse LaunchAuto()
        {
            return Launch(AlyUpdateUiMode.Auto);
        }

        /// <summary>交互式检查（用户点击「检查更新」）。</summary>
        public AlyResponse LaunchCheck()
        {
            return Launch(AlyUpdateUiMode.Check);
        }

        /// <summary>直接更新（下载 + 应用，不再询问）。</summary>
        public AlyResponse LaunchUpdate()
        {
            return Launch(AlyUpdateUiMode.Update);
        }

        /// <summary>打开回滚版本选择窗口。</summary>
        public AlyResponse LaunchRollback()
        {
            return Launch(AlyUpdateUiMode.Rollback);
        }

        /// <summary>静默检查（无更新时不打扰用户，有更新才显示）。</summary>
        public AlyResponse LaunchSilent()
        {
            return Launch(AlyUpdateUiMode.Silent);
        }

        /// <summary>模式枚举 → 命令行 -mode 取值。</summary>
        public static string ToArgument(AlyUpdateUiMode mode)
        {
            switch (mode)
            {
                case AlyUpdateUiMode.Check:
                    return "check";
                case AlyUpdateUiMode.Update:
                    return "update";
                case AlyUpdateUiMode.Rollback:
                    return "rollback";
                case AlyUpdateUiMode.Silent:
                    return "silent";
                default:
                    return "auto";
            }
        }

        private void OnProcessExited(object sender, EventArgs e)
        {
            var process = sender as Process;
            int exitCode = 0;
            try
            {
                if (process != null) exitCode = process.ExitCode;
            }
            catch
            {
                // 进程句柄已释放等异常：退出码按 0 处理，不影响宿主
            }

            lock (_sync)
            {
                _lastExitCode = exitCode;
                // 进程已退出：立刻释放句柄，避免每次检查泄漏一个 Process 对象
                if (ReferenceEquals(_process, process)) _process = null;
                DisposeProcess(process);
            }

            var handler = Exited;
            if (handler != null) handler(exitCode);
        }

        /// <summary>取消事件订阅并释放 Process（幂等，忽略已释放/已退出等异常）。</summary>
        private void DisposeProcess(Process process)
        {
            if (process == null) return;
            try { process.Exited -= OnProcessExited; } catch { }
            try { process.Dispose(); } catch { }
        }

        private void RaiseStarted(int pid)
        {
            var handler = Started;
            if (handler != null) handler(pid);
        }

        private static bool SafeHasExited(Process process)
        {
            try
            {
                return process.HasExited;
            }
            catch
            {
                return true;
            }
        }

        public void Dispose()
        {
            if (_disposed) return;
            _disposed = true;

            lock (_sync)
            {
                var process = _process;
                _process = null;
                DisposeProcess(process);
            }
        }
    }
}
