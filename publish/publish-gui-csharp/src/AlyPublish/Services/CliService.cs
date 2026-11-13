using System.Globalization;
using AlyPublish.Constants;
using AlyPublish.Helpers;
using AlyPublish.Localization;
using AlyPublish.Models.Cli;
using Newtonsoft.Json;
using Serilog;

namespace AlyPublish.Services;

/// <summary>
/// Typed facade over <c>aly-publish</c>. Every method either returns real data or throws
/// <see cref="CliException"/>; cancellations surface as <see cref="OperationCanceledException"/>.
/// </summary>
public sealed class CliService
{
    private const int DefaultTimeoutMilliseconds = 60_000;
    private const int StatusTimeoutMilliseconds = 120_000;
    // 审查 M16：push 的总超时不能短于 CLI 自身的重试预算。
    // CLI 单请求 600s、分片/直传各 5 次重试，默认分片 128KiB——
    // 1GB 发布包在 1MB/s 链路上需要约 1050s，旧的 300s 会在传输中途杀掉进程，
    // 用户只看到"超时"。这里放到 30 分钟，覆盖典型弱网大包场景。
    private const int PushTimeoutMilliseconds = 30 * 60_000;

    public CliService()
    {
        ExecutablePath = FindExecutable();
        Log.Information("aly-publish: {Path}", IsAvailable ? ExecutablePath : "未找到");
    }

    public string ExecutablePath { get; private set; }

    public bool IsAvailable => !string.IsNullOrEmpty(ExecutablePath);

    /// <summary>Re-runs discovery after the user installs the CLI next to the application.</summary>
    public void RefreshAvailability()
    {
        ExecutablePath = FindExecutable();
    }

    public Task<StatusData> GetStatusAsync(string projectPath, CancellationToken cancellationToken = default)
        => RunForDataAsync<StatusData>(["status"], projectPath, StatusTimeoutMilliseconds, cancellationToken);

    public async Task<List<ChangeLog>> GetLogAsync(
        string projectPath,
        int limit = 20,
        CancellationToken cancellationToken = default)
    {
        var output = await RunCoreAsync<List<ChangeLog>>(
            ["log", "--limit", limit.ToString(CultureInfo.InvariantCulture)],
            projectPath,
            DefaultTimeoutMilliseconds,
            cancellationToken);
        return output.Data ?? [];
    }

    public Task AddAllAsync(string projectPath, CancellationToken cancellationToken = default)
        => RunAsync(["add", "--all"], projectPath, DefaultTimeoutMilliseconds, cancellationToken);

    public Task ResetAllAsync(string projectPath, CancellationToken cancellationToken = default)
        => RunAsync(["reset", "--all"], projectPath, DefaultTimeoutMilliseconds, cancellationToken);

    public Task AddFilesAsync(
        string projectPath,
        IReadOnlyList<string> files,
        CancellationToken cancellationToken = default)
        => RunAsync(WithFiles("add", files), projectPath, DefaultTimeoutMilliseconds, cancellationToken);

    public Task ResetFilesAsync(
        string projectPath,
        IReadOnlyList<string> files,
        CancellationToken cancellationToken = default)
        => RunAsync(WithFiles("reset", files), projectPath, DefaultTimeoutMilliseconds, cancellationToken);

    public async Task<List<ProjectInfo>> GetServerProjectsAsync(
        string serverUrl,
        CancellationToken cancellationToken = default)
    {
        var output = await RunCoreAsync<List<ProjectInfo>>(
            ["project", "list", "--server", serverUrl],
            string.Empty,
            DefaultTimeoutMilliseconds,
            cancellationToken);
        return output.Data ?? [];
    }

    public Task<ProjectInfo> CreateServerProjectAsync(
        string serverUrl,
        string name,
        string title,
        bool forceUpdate,
        IReadOnlyList<string> ignoreFolders,
        IReadOnlyList<string> ignoreFiles,
        CancellationToken cancellationToken = default)
    {
        var arguments = new List<string> { "project", "create", "--server", serverUrl, "--name", name, "--title", title };
        AddFlag(arguments, "--force-update", forceUpdate);
        AddCsv(arguments, "--ignore-folders", ignoreFolders);
        AddCsv(arguments, "--ignore-files", ignoreFiles);

        return RunForDataAsync<ProjectInfo>(arguments, string.Empty, DefaultTimeoutMilliseconds, cancellationToken);
    }

    public Task UpdateServerProjectAsync(
        string serverUrl,
        string name,
        string title,
        bool forceUpdate,
        IReadOnlyList<string> ignoreFolders,
        IReadOnlyList<string> ignoreFiles,
        CancellationToken cancellationToken = default)
    {
        var arguments = new List<string> { "project", "update", "--server", serverUrl, "--name", name, "--title", title };
        AddFlag(arguments, "--force-update", forceUpdate);
        AddCsv(arguments, "--ignore-folders", ignoreFolders);
        AddCsv(arguments, "--ignore-files", ignoreFiles);

        return RunAsync(arguments, string.Empty, DefaultTimeoutMilliseconds, cancellationToken);
    }

    public Task InitializeProjectAsync(
        string projectPath,
        string serverUrl,
        string projectName,
        IReadOnlyList<string> ignoreFolders,
        IReadOnlyList<string> ignoreFiles,
        IReadOnlyList<string> unCopyFolders,
        IReadOnlyList<string> unCopyFiles,
        CancellationToken cancellationToken = default)
    {
        var arguments = new List<string> { "config", "init", "--server", serverUrl, "--project", projectName };
        AddCsv(arguments, "--ignore-folders", ignoreFolders);
        AddCsv(arguments, "--ignore-files", ignoreFiles);
        AddCsv(arguments, "--un-copy-folders", unCopyFolders);
        AddCsv(arguments, "--un-copy-files", unCopyFiles);

        return RunAsync(arguments, projectPath, DefaultTimeoutMilliseconds, cancellationToken);
    }

    public Task SetConfigAsync(
        string projectPath,
        string key,
        string value,
        CancellationToken cancellationToken = default)
        => RunAsync(["config", "set", key, value], projectPath, DefaultTimeoutMilliseconds, cancellationToken);

    public Task AddConfigArrayItemAsync(
        string projectPath,
        string key,
        string item,
        CancellationToken cancellationToken = default)
        => RunAsync(
            ["config", "set-array", key, "--add", item],
            projectPath,
            DefaultTimeoutMilliseconds,
            cancellationToken);

    public Task RemoveConfigArrayItemAsync(
        string projectPath,
        string key,
        string item,
        CancellationToken cancellationToken = default)
        => RunAsync(
            ["config", "set-array", key, "--remove", item],
            projectPath,
            DefaultTimeoutMilliseconds,
            cancellationToken);

    /// <summary>Pushes staged files and reports per-file progress as the CLI streams it.</summary>
    public async Task PushAsync(
        string projectPath,
        string version,
        string message,
        string afterApplyUpdateScript,
        bool setForceUpdate,
        bool clearForceUpdate,
        IProgress<UploadProgress> progress,
        CancellationToken cancellationToken = default)
    {
        EnsureAvailable(["push"]);

        var arguments = new List<string>
        {
            AppConstants.JsonOutputArgument,
            "push",
            "--version",
            version,
            "--message",
            message
        };
        // 审查 B7：CLI 的强制更新是三态（--set / --clear / 都不传），GUI 的复选框
        // 必须始终翻译成显式的一侧——否则"取消勾选"会退化为"不传"，服务端保留
        // 旧的 true，用户看到的"已取消强制更新"并不会生效。
        AddFlag(arguments, "--set-force-update", setForceUpdate);
        AddFlag(arguments, "--clear-force-update", clearForceUpdate);
        if (!string.IsNullOrWhiteSpace(afterApplyUpdateScript))
        {
            arguments.Add("--after-apply-update-script");
            arguments.Add(afterApplyUpdateScript);
        }

        Log.Information("CLI 输入: {Executable} {Arguments} (工作目录={WorkingDirectory})",
            ExecutablePath, string.Join(' ', arguments),
            string.IsNullOrWhiteSpace(projectPath) ? "(当前目录)" : projectPath);

        string? failureMessage = null;
        var hasResult = false;
        var processResult = await ProcessService.RunWithProgressAsync(
            ExecutablePath,
            arguments,
            line =>
            {
                Log.Information("CLI 输出: {Line}", line);
                var parsed = PushResultParser.Parse(line);
                if (parsed.Kind == PushLineKind.Progress && parsed.Progress is not null)
                {
                    progress.Report(parsed.Progress);
                }

                if (parsed.ErrorMessage is not null)
                {
                    failureMessage = parsed.ErrorMessage;
                }

                if (parsed.Kind == PushLineKind.Success)
                {
                    hasResult = true;
                }
            },
            string.IsNullOrWhiteSpace(projectPath) ? null : projectPath,
            PushTimeoutMilliseconds,
            cancellationToken);

        Log.Information("CLI 结果: 成功={Success}", processResult.Success);
        Log.Information("CLI 标准错误: {Error}", processResult.StandardError);

        if (processResult.Canceled)
        {
            throw new OperationCanceledException(cancellationToken);
        }

        if (!string.IsNullOrWhiteSpace(failureMessage))
        {
            throw new CliException(failureMessage);
        }

        if (!processResult.Success)
        {
            var error = string.IsNullOrWhiteSpace(processResult.StandardError)
                ? LanguageManager.T(LanguageManager.Instance.Message_Cli_PushExited)
                : processResult.StandardError;
            throw new CliException(error);
        }

        if (!hasResult)
        {
            throw new CliException(LanguageManager.T(LanguageManager.Instance.Message_Cli_PushInvalid));
        }

        Log.Information("推送成功: {Version}", version);
    }

    private async Task<T> RunForDataAsync<T>(
        IReadOnlyList<string> arguments,
        string projectPath,
        int timeoutMilliseconds,
        CancellationToken cancellationToken)
    {
        var output = await RunCoreAsync<T>(arguments, projectPath, timeoutMilliseconds, cancellationToken);
        return output.Data ?? throw new CliException(LanguageManager.T(LanguageManager.Instance.Message_Cli_NoData));
    }

    private async Task RunAsync(
        IReadOnlyList<string> arguments,
        string projectPath,
        int timeoutMilliseconds,
        CancellationToken cancellationToken)
        => await RunCoreAsync<object>(arguments, projectPath, timeoutMilliseconds, cancellationToken);

    private async Task<CliOutput<T>> RunCoreAsync<T>(
        IReadOnlyList<string> arguments,
        string projectPath,
        int timeoutMilliseconds,
        CancellationToken cancellationToken)
    {
        EnsureAvailable(arguments);

        var fullArguments = new List<string>(arguments.Count + 1) { AppConstants.JsonOutputArgument };
        fullArguments.AddRange(arguments);
        var workingDirectory = string.IsNullOrWhiteSpace(projectPath) ? null : projectPath;

        Log.Information("CLI 输入: {Executable} {Arguments} (工作目录={WorkingDirectory})",
            ExecutablePath, string.Join(' ', fullArguments), workingDirectory ?? "(当前目录)");

        var result = await ProcessService.RunAsync(
            ExecutablePath,
            fullArguments,
            workingDirectory,
            timeoutMilliseconds,
            cancellationToken);

        Log.Information("CLI 结果: 成功={Success}; 工作目录={WorkingDirectory}", result.Success, workingDirectory ?? "(当前目录)");
        foreach (var line in (result.StandardOutput ?? string.Empty).Split('\n'))
        {
            Log.Information("CLI 标准输出: {Line}", line);
        }

        foreach (var line in (result.StandardError ?? string.Empty).Split('\n'))
        {
            Log.Information("CLI 标准错误: {Line}", line);
        }

        if (result.Canceled)
        {
            throw new OperationCanceledException(cancellationToken);
        }

        if (!result.Success)
        {
            var error = string.IsNullOrWhiteSpace(result.StandardError)
                ? LanguageManager.T(LanguageManager.Instance.Message_Cli_Failed)
                : result.StandardError;
            throw new CliException(error);
        }

        if (string.IsNullOrWhiteSpace(result.StandardOutput))
        {
            throw new CliException(LanguageManager.T(LanguageManager.Instance.Message_Cli_NoOutput));
        }

        CliOutput<T>? output;
        try
        {
            output = JsonConvert.DeserializeObject<CliOutput<T>>(result.StandardOutput);
        }
        catch (JsonException ex)
        {
            Log.Error(ex, "解析 CLI 输出失败: {Output}", result.StandardOutput);
            throw new CliException(LanguageManager.T(LanguageManager.Instance.Message_Cli_Unparsable), ex);
        }

        if (output is null)
        {
            throw new CliException(LanguageManager.T(LanguageManager.Instance.Message_Cli_NoResult));
        }

        if (!output.IsSuccess)
        {
            throw new CliException(
                string.IsNullOrWhiteSpace(output.ErrorMsg)
                    ? LanguageManager.T(LanguageManager.Instance.Message_Cli_Failed)
                    : output.ErrorMsg);
        }

        return output;
    }

    private void EnsureAvailable(IReadOnlyList<string> arguments)
    {
        if (IsAvailable)
        {
            return;
        }

        Log.Error("未找到 aly-publish，无法执行: {Arguments}", string.Join(' ', arguments));
        throw new CliException(LanguageManager.T(LanguageManager.Instance.Message_Cli_NotFound));
    }

    private static List<string> WithFiles(string command, IReadOnlyList<string> files)
    {
        var arguments = new List<string>(files.Count + 2) { command, "--" };
        arguments.AddRange(files);
        return arguments;
    }

    private static void AddFlag(List<string> arguments, string flag, bool enabled)
    {
        if (enabled)
        {
            arguments.Add(flag);
        }
    }

    private static void AddCsv(List<string> arguments, string flag, IReadOnlyList<string> values)
    {
        if (values.Count == 0)
        {
            return;
        }

        arguments.Add(flag);
        arguments.Add(string.Join(',', values));
    }

    private static string FindExecutable()
    {
        var baseDirectory = AppDomain.CurrentDomain.BaseDirectory;
        var candidates = PkHelper.PubProbes(baseDirectory, string.Empty, PkHelper.PubRids(baseDirectory));

        foreach (var candidate in candidates)
        {
            if (!File.Exists(candidate.FullName))
            {
                continue;
            }

            Log.Information("找到 aly-publish: {Path} (rid={Rid})", candidate.FullName, candidate.Rid);
            return candidate.FullName;
        }

        var missing = PkHelper.ListMissing(baseDirectory);
        Log.Warning(
            "未找到 aly-publish（安装包缺少 {Folder}/ 目录，或未放在程序目录）: {Directory}；payload 缺失 {Count} 项: {List}",
            PkHelper.PubFolderName,
            baseDirectory,
            missing.Count,
            string.Join(';', missing));
        return string.Empty;
    }
}

