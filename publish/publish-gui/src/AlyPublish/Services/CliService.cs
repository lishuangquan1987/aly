using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Newtonsoft.Json;
using AlyPublish.Models.Cli;
using Serilog;

namespace AlyPublish.Services;

public class CliService
{
    private readonly ProcessService _ps;

    public string CliPath => FindCliDefault();
    

    public CliService(ProcessService ps)
    {
        _ps = ps;
        Log.Debug("CliService 初始化: Found={Found}, Path={Path}", Found, CliPath);
    }

    public bool Found => !string.IsNullOrEmpty(CliPath) && File.Exists(CliPath);

    private static string FindCliDefault()
    {
        var exeDir = AppDomain.CurrentDomain.BaseDirectory;
        var exeName = OperatingSystem.IsWindows() ? "aly-publish.exe" : "aly-publish";
        Log.Debug("查找 aly-publish: BaseDirectory={Dir}", exeDir);

        var same = Path.Combine(exeDir, exeName);
        if (File.Exists(same))
        {
            Log.Debug("找到 aly-publish: {Path}", same);
            return same;
        }

        var rel = Path.GetFullPath(Path.Combine(exeDir, "..", "..", "..", "..", "publish-cli", exeName));
        if (File.Exists(rel))
        {
            Log.Debug("找到 aly-publish (开发路径): {Path}", rel);
            return rel;
        }

        Log.Warning("未找到 aly-publish");
        return string.Empty;
    }

    public string? FindCli() => Found ? CliPath : null;

    // ── Generic runner ──────────────────────────────────

    public async Task<CliOutput<T>?> RunAsync<T>(string args, string projectPath, int timeoutMs = 30000)
    {
        if (!Found)
        {
            Log.Error("aly-publish 未找到，无法执行命令: {Args}", args);
            return Fail<T>("未找到 aly-publish");
        }

        var fullArgs = $"{args} --json";
        var workDir = string.IsNullOrWhiteSpace(projectPath) ? null : projectPath;
        Log.Debug("执行 CLI: {Cli} {Args} (WorkDir={WorkDir})", CliPath, fullArgs, workDir ?? "(无)");

        var result = await _ps.RunAsync(CliPath, fullArgs, workDir, timeoutMs: timeoutMs);

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
            return Fail<T>("aly-publish 无输出");
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
        => RunAsync<object>($"add {string.Join(" ", files.Select(f => $"\"{f.Replace("\"", "\\\"")}\""))}", projectPath);

    public Task<CliOutput<object>?> AddAllAsync(string projectPath)
        => RunAsync<object>("add --all", projectPath);

    public Task<CliOutput<object>?> ResetAllAsync(string projectPath)
        => RunAsync<object>("reset --all", projectPath);

    public Task<CliOutput<object>?> PushAsync(string projectPath, string version, string message, string afterApplyUpdateScript = "", bool setForceUpdate = false)
    {
        var args = $"push --version \"{version}\" --message \"{message}\"";
        if (setForceUpdate)
            args += " --set-force-update";
        if (!string.IsNullOrWhiteSpace(afterApplyUpdateScript))
            args += $" --after-apply-update-script \"{afterApplyUpdateScript.Replace("\"", "\\\"")}\"";
        return RunAsync<object>(args, projectPath, 120000);
    }

    public Task<CliOutput<List<ChangeLog>>?> GetLogAsync(string projectPath, int limit = 20)
        => RunAsync<List<ChangeLog>>($"log --limit {limit}", projectPath);

    public Task<CliOutput<object>?> ConfigInitAsync(string projectPath, string serverUrl, string projectName, string ignoreFolders = "", string ignoreFiles = "")
    {
        var args = $"config init --server \"{serverUrl}\" --project \"{projectName}\"";
        if (!string.IsNullOrWhiteSpace(ignoreFolders))
            args += $" --ignore-folders \"{ignoreFolders.Replace("\"", "\\\"")}\"";
        if (!string.IsNullOrWhiteSpace(ignoreFiles))
            args += $" --ignore-files \"{ignoreFiles.Replace("\"", "\\\"")}\"";
        return RunAsync<object>(args, projectPath);
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

    public Task<CliOutput<object>?> ProjectUpdateAsync(
        string serverUrl, string name, string title, bool forceUpdate,
        List<string>? ignoreFolders = null, List<string>? ignoreFiles = null)
    {
        Log.Information("更新服务端项目: ServerUrl={Url}, Name={Name}, IgnoreFolders={Folders}, IgnoreFiles={Files}",
            serverUrl, name, ignoreFolders?.Count ?? 0, ignoreFiles?.Count ?? 0);
        var args = $"project update --server \"{serverUrl}\" --name \"{name}\" --title \"{title}\"";
        if (forceUpdate) args += " --force-update";
        if (ignoreFolders is { Count: > 0 }) args += $" --ignore-folders \"{string.Join(",", ignoreFolders)}\"";
        if (ignoreFiles is { Count: > 0 }) args += $" --ignore-files \"{string.Join(",", ignoreFiles)}\"";
        return RunAsync<object>(args, string.Empty);
    }

    // ── Config management (via CLI, never directly touches .updator/) ──

    public Task<CliOutput<object>?> ConfigSetAsync(string projectPath, string key, string value)
    {
        Log.Information("CLI config set: Key={Key}, Value={Value}, Path={Path}", key, value, projectPath);
        return RunAsync<object>($"config set {key} \"{value}\"", projectPath);
    }

    public Task<CliOutput<object>?> ConfigSetArrayAddAsync(string projectPath, string key, string item)
    {
        Log.Information("CLI config set-array add: Key={Key}, Item={Item}, Path={Path}", key, item, projectPath);
        return RunAsync<object>($"config set-array {key} --add \"{item}\"", projectPath);
    }

    public Task<CliOutput<object>?> ConfigSetArrayRemoveAsync(string projectPath, string key, string item)
    {
        Log.Information("CLI config set-array remove: Key={Key}, Item={Item}, Path={Path}", key, item, projectPath);
        return RunAsync<object>($"config set-array {key} --remove \"{item}\"", projectPath);
    }
}
