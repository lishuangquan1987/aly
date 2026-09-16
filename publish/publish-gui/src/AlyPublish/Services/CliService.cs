using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;
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

    /// <summary>
    /// 使用 ArgumentList 的通用执行器：参数不拼接成字符串，由 ProcessStartInfo.ArgumentList
    /// 按 Windows 规则自动转义，杜绝用户输入（如发布说明含双引号）注入 CLI 参数。
    /// </summary>
    public async Task<CliOutput<T>?> RunListAsync<T>(IReadOnlyList<string> args, string projectPath, int timeoutMs = 30000)
    {
        if (!Found)
        {
            Log.Error("aly-publish 未找到，无法执行命令: {Args}", string.Join(" ", args));
            return Fail<T>("未找到 aly-publish");
        }

        var list = new List<string>(args) { "--json" };
        var workDir = string.IsNullOrWhiteSpace(projectPath) ? null : projectPath;
        Log.Debug("执行 CLI (ArgumentList): {Cli} {Args} (WorkDir={WorkDir})",
            CliPath, string.Join(" ", list), workDir ?? "(无)");

        var result = await _ps.RunAsync(CliPath, list, workDir, timeoutMs: timeoutMs);

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
    {
        var args = new List<string> { "add" };
        args.AddRange(files);
        return RunListAsync<object>(args, projectPath);
    }

    public Task<CliOutput<object>?> AddAllAsync(string projectPath)
        => RunAsync<object>("add --all", projectPath);

    public Task<CliOutput<object>?> ResetAllAsync(string projectPath)
        => RunAsync<object>("reset --all", projectPath);

    /// <summary>只取消暂存指定文件（CLI reset &lt;file&gt;...），避免 reset --all 后重加失败丢全部暂存。</summary>
    public Task<CliOutput<object>?> ResetFilesAsync(string projectPath, List<string> files)
    {
        var args = new List<string> { "reset" };
        args.AddRange(files);
        return RunListAsync<object>(args, projectPath);
    }

    public Task<CliOutput<object>?> PushAsync(string projectPath, string version, string message, string afterApplyUpdateScript = "", bool setForceUpdate = false)
    {
        var args = BuildPushArgsList(version, message, afterApplyUpdateScript, setForceUpdate);
        return RunListAsync<object>(args, projectPath, 300000);
    }

    /// <summary>
    /// 推送并实时回调上传进度（每行 stdout 解析为 UploadProgress 后调用 onProgress）。
    /// </summary>
    public async Task<CliOutput<object>?> PushWithProgressAsync(
        string projectPath, string version, string message,
        Action<UploadProgress> onProgress,
        string afterApplyUpdateScript = "", bool setForceUpdate = false)
    {
        if (!Found)
        {
            Log.Error("aly-publish 未找到，无法执行推送");
            return Fail<object>("未找到 aly-publish");
        }

        var args = BuildPushArgsList(version, message, afterApplyUpdateScript, setForceUpdate);
        args.Add("--json");
        var workDir = string.IsNullOrWhiteSpace(projectPath) ? null : projectPath;

        CliOutput<object>? finalResult = null;
        // isSuccess=false 的失败结果单独保存：它后面会紧跟 progressDone 哨兵行，
        // 若直接覆盖 finalResult，失败原因会被吞掉，界面将误判为发布成功。
        CliOutput<object>? failResult = null;

        var procResult = await _ps.RunWithProgressAsync(CliPath, args, line =>
        {
            try
            {
                // 用 JObject 检查 data 是否包含 "status" 字段来区分进度行和最终结果
                var jObj = JObject.Parse(line);
                var hasStatus = jObj["data"]?["status"] != null;

                if (hasStatus)
                {
                    // 进度行：START / DONE / FAIL（FAIL 行的 data.error 含具体失败原因）
                    var output = JsonConvert.DeserializeObject<CliOutput<UploadProgress>>(line);
                    if (output?.Data != null)
                    {
                        onProgress(output.Data);
                    }
                    return;
                }

                // 非进度行：isSuccess=false 的失败结果、成功结果 (data: {"version","files"})、
                // 或 progressDone 哨兵 (isSuccess:true, data:null)
                var result = JsonConvert.DeserializeObject<CliOutput<object>>(line);
                if (result == null)
                    return;

                if (!result.IsSuccess)
                {
                    // 失败结果：解析并保留 errorMsg 作为明确的失败原因
                    failResult = result;
                }
                else if (result.Data != null)
                {
                    // 成功结果：data 非空（version/files）
                    finalResult = result;
                }
                // isSuccess:true 且 data:null 的哨兵行：不覆盖已有结果
            }
            catch (JsonException ex)
            {
                Log.Warning(ex, "进度行 JSON 解析失败: {Line}", line);
            }
        }, workDir, 300000);

        if (!procResult.Success)
        {
            // 进程级失败（启动失败/超时/非零退出）：优先用 CLI 已输出的失败原因
            if (failResult != null && !string.IsNullOrWhiteSpace(failResult.ErrorMsg))
                return failResult;

            var stderrMsg = string.IsNullOrWhiteSpace(procResult.StandardError) ? null : procResult.StandardError;
            Log.Warning("CLI push 失败: {Error}", stderrMsg ?? "(无 stderr)");
            return stderrMsg != null
                ? Fail<object>($"推送失败: {stderrMsg}")
                : Fail<object>("推送失败：CLI 进程执行失败或超时，请查看日志");
        }

        // 有失败结果（isSuccess=false 的 errorMsg）优先返回
        if (failResult != null)
            return failResult;
        if (finalResult != null)
            return finalResult;

        Log.Warning("CLI push 无有效结果输出");
        return Fail<object>("推送失败：CLI 未返回有效结果，请查看日志");
    }

    private static List<string> BuildPushArgsList(string version, string message, string afterApplyUpdateScript, bool setForceUpdate)
    {
        var args = new List<string> { "push", "--version", version, "--message", message };
        if (setForceUpdate)
            args.Add("--set-force-update");
        if (!string.IsNullOrWhiteSpace(afterApplyUpdateScript))
        {
            args.Add("--after-apply-update-script");
            args.Add(afterApplyUpdateScript);
        }
        return args;
    }

    public Task<CliOutput<List<ChangeLog>>?> GetLogAsync(string projectPath, int limit = 20)
        => RunAsync<List<ChangeLog>>($"log --limit {limit}", projectPath);

    public Task<CliOutput<object>?> ConfigInitAsync(string projectPath, string serverUrl, string projectName, string ignoreFolders = "", string ignoreFiles = "")
    {
        var args = new List<string> { "config", "init", "--server", serverUrl, "--project", projectName };
        if (!string.IsNullOrWhiteSpace(ignoreFolders))
        {
            args.Add("--ignore-folders");
            args.Add(ignoreFolders);
        }
        if (!string.IsNullOrWhiteSpace(ignoreFiles))
        {
            args.Add("--ignore-files");
            args.Add(ignoreFiles);
        }
        return RunListAsync<object>(args, projectPath);
    }

    public Task<CliOutput<List<ProjectInfo>>?> ProjectListAsync(string serverUrl)
    {
        Log.Information("获取服务端项目列表: ServerUrl={Url}", serverUrl);
        return RunListAsync<List<ProjectInfo>>(
            new List<string> { "project", "list", "--server", serverUrl }, string.Empty);
    }

    public Task<CliOutput<ProjectInfo>?> ProjectCreateAsync(
        string serverUrl, string name, string title, bool forceUpdate,
        List<string>? ignoreFolders = null, List<string>? ignoreFiles = null)
    {
        Log.Information("创建服务端项目: ServerUrl={Url}, Name={Name}, Title={Title}, ForceUpdate={Force}",
            serverUrl, name, title, forceUpdate);
        var args = new List<string> { "project", "create", "--server", serverUrl, "--name", name, "--title", title };
        if (forceUpdate) args.Add("--force-update");
        if (ignoreFolders is { Count: > 0 }) { args.Add("--ignore-folders"); args.Add(string.Join(",", ignoreFolders)); }
        if (ignoreFiles is { Count: > 0 }) { args.Add("--ignore-files"); args.Add(string.Join(",", ignoreFiles)); }
        return RunListAsync<ProjectInfo>(args, string.Empty);
    }

    public Task<CliOutput<object>?> ProjectUpdateAsync(
        string serverUrl, string name, string title, bool forceUpdate,
        List<string>? ignoreFolders = null, List<string>? ignoreFiles = null)
    {
        Log.Information("更新服务端项目: ServerUrl={Url}, Name={Name}, IgnoreFolders={Folders}, IgnoreFiles={Files}",
            serverUrl, name, ignoreFolders?.Count ?? 0, ignoreFiles?.Count ?? 0);
        var args = new List<string> { "project", "update", "--server", serverUrl, "--name", name, "--title", title };
        if (forceUpdate) args.Add("--force-update");
        if (ignoreFolders is { Count: > 0 }) { args.Add("--ignore-folders"); args.Add(string.Join(",", ignoreFolders)); }
        if (ignoreFiles is { Count: > 0 }) { args.Add("--ignore-files"); args.Add(string.Join(",", ignoreFiles)); }
        return RunListAsync<object>(args, string.Empty);
    }

    // ── Config management (via CLI, never directly touches .updator/) ──

    public Task<CliOutput<object>?> ConfigSetAsync(string projectPath, string key, string value)
    {
        Log.Information("CLI config set: Key={Key}, Value={Value}, Path={Path}", key, value, projectPath);
        return RunListAsync<object>(new List<string> { "config", "set", key, value }, projectPath);
    }

    public Task<CliOutput<object>?> ConfigSetArrayAddAsync(string projectPath, string key, string item)
    {
        Log.Information("CLI config set-array add: Key={Key}, Item={Item}, Path={Path}", key, item, projectPath);
        return RunListAsync<object>(new List<string> { "config", "set-array", key, "--add", item }, projectPath);
    }

    public Task<CliOutput<object>?> ConfigSetArrayRemoveAsync(string projectPath, string key, string item)
    {
        Log.Information("CLI config set-array remove: Key={Key}, Item={Item}, Path={Path}", key, item, projectPath);
        return RunListAsync<object>(new List<string> { "config", "set-array", key, "--remove", item }, projectPath);
    }
}
