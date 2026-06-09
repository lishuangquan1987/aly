using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Newtonsoft.Json;
using PublishGui.Models.Cli;
using Serilog;

namespace PublishGui.Services;

public class CliService
{
    private readonly ProcessService _ps;
    private string _cliPath;

    public string CliPath
    {
        get => _cliPath;
        set => _cliPath = value;
    }

    public CliService(ProcessService ps)
    {
        _ps = ps;
        _cliPath = FindCliDefault();
        Log.Debug("CliService 初始化: Found={Found}, Path={Path}", Found, _cliPath);
    }

    public bool Found => !string.IsNullOrEmpty(_cliPath) && File.Exists(_cliPath);

    private static string FindCliDefault()
    {
        var exeDir = AppDomain.CurrentDomain.BaseDirectory;
        var exeName = OperatingSystem.IsWindows() ? "publish-cli.exe" : "publish-cli";
        Log.Debug("查找 publish-cli: BaseDirectory={Dir}", exeDir);

        var same = Path.Combine(exeDir, exeName);
        if (File.Exists(same))
        {
            Log.Debug("找到 publish-cli: {Path}", same);
            return same;
        }

        var rel = Path.GetFullPath(Path.Combine(exeDir, "..", "..", "..", "..", "publish-cli", exeName));
        if (File.Exists(rel))
        {
            Log.Debug("找到 publish-cli (相对路径): {Path}", rel);
            return rel;
        }

        Log.Warning("未找到 publish-cli");
        return string.Empty;
    }

    public string? FindCli() => Found ? _cliPath : null;

    // ── Generic runner ──────────────────────────────────

    public async Task<CliOutput<T>?> RunAsync<T>(string args, string projectPath, int timeoutMs = 30000)
    {
        if (!Found)
        {
            Log.Error("publish-cli.exe 未找到，无法执行命令: {Args}", args);
            return Fail<T>("未找到 publish-cli.exe");
        }

        var safePath = projectPath ?? string.Empty;
        var fullArgs = $"{args} --json --path \"{safePath.TrimEnd('\\', '/')}\"";
        Log.Debug("执行 CLI: {Cli} {Args}", _cliPath, fullArgs);

        var result = await _ps.RunAsync(_cliPath, fullArgs, timeoutMs: timeoutMs);

        Log.Information("CLI 执行结果: Success={Success}, ExitCode={Code}, StdOut长度={OutLen}, StdErr={Err}",
            result.Success, result.ExitCode, result.StandardOutput?.Length ?? 0,
            string.IsNullOrEmpty(result.StandardError) ? "(无)" : result.StandardError);

        if (!result.Success)
        {
            var errMsg = string.IsNullOrEmpty(result.StandardError) ? "执行失败" : result.StandardError;
            Log.Warning("CLI 命令失败: {Error}", errMsg);
            return Fail<T>(errMsg);
        }

        if (string.IsNullOrWhiteSpace(result.StandardOutput))
        {
            Log.Warning("CLI 无输出");
            return Fail<T>("publish-cli 无输出");
        }

        try
        {
            Log.Debug("CLI 输出 (前500字符): {Output}", result.StandardOutput.Length > 500
                ? result.StandardOutput[..500] + "..."
                : result.StandardOutput);
            return JsonConvert.DeserializeObject<CliOutput<T>>(result.StandardOutput);
        }
        catch (JsonException ex)
        {
            Log.Error(ex, "JSON 解析失败: {Output}", result.StandardOutput);
            return Fail<T>($"JSON 解析失败: {ex.Message}");
        }
    }

    private static CliOutput<T> Fail<T>(string msg) => new() { IsSuccess = false, ErrorMsg = msg };

    // ── Commands ─────────────────────────────────────────

    public Task<CliOutput<StatusData>?> GetStatusAsync(string projectPath)
        => RunAsync<StatusData>("status", projectPath);

    public Task<CliOutput<object>?> AddFilesAsync(string projectPath, List<string> files)
        => RunAsync<object>($"add {string.Join(" ", files.Select(f => $"\"{f}\""))}", projectPath);

    public Task<CliOutput<object>?> AddAllAsync(string projectPath)
        => RunAsync<object>("add --all", projectPath);

    public Task<CliOutput<object>?> ResetAllAsync(string projectPath)
        => RunAsync<object>("reset --all", projectPath);

    public Task<CliOutput<object>?> PushAsync(string projectPath, string version, string message)
        => RunAsync<object>($"push --version \"{version}\" --message \"{message}\"", projectPath, 120000);

    public Task<CliOutput<object>?> PublishAsync(string projectPath, string version, string message)
        => RunAsync<object>($"publish --version \"{version}\" --message \"{message}\"", projectPath, 120000);

    public Task<CliOutput<List<ChangeLog>>?> GetLogAsync(string projectPath, int limit = 20)
        => RunAsync<List<ChangeLog>>($"log --limit {limit}", projectPath);

    public Task<CliOutput<object>?> ConfigInitAsync(string projectPath, string serverUrl, string projectName, int projectId)
    {
        var safePath = projectPath ?? string.Empty;
        var trimmedPath = safePath.TrimEnd('\\', '/');
        var args = $"config init --server \"{serverUrl}\" --project \"{projectName}\" --path \"{trimmedPath}\"";
        if (projectId > 0) args += $" --id {projectId}";
        return RunAsync<object>(args, safePath);
    }

    public Task<CliOutput<List<ProjectInfo>>?> ProjectListAsync(string serverUrl)
    {
        Log.Information("获取服务端项目列表: ServerUrl={Url}", serverUrl);
        return RunAsync<List<ProjectInfo>>($"project list --server \"{serverUrl}\"", string.Empty);
    }

    public Task<CliOutput<ProjectInfo>?> ProjectCreateAsync(
        string serverUrl, string name, string title, bool forceUpdate,
        List<string>? ignoreFolders = null, List<string>? ignoreFiles = null)
    {
        Log.Information("创建服务端项目: ServerUrl={Url}, Name={Name}, Title={Title}, ForceUpdate={Force}",
            serverUrl, name, title, forceUpdate);
        var args = $"project create --server \"{serverUrl}\" --name \"{name}\" --title \"{title}\"";
        if (forceUpdate) args += " --force-update";
        if (ignoreFolders is { Count: > 0 }) args += $" --ignore-folders \"{string.Join(",", ignoreFolders)}\"";
        if (ignoreFiles is { Count: > 0 }) args += $" --ignore-files \"{string.Join(",", ignoreFiles)}\"";
        return RunAsync<ProjectInfo>(args, string.Empty);
    }
}
