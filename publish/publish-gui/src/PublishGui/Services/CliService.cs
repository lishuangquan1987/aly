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
    }

    public bool Found => !string.IsNullOrEmpty(_cliPath) && File.Exists(_cliPath);

    private static string FindCliDefault()
    {
        var exeDir = AppDomain.CurrentDomain.BaseDirectory;
        // Check same dir as GUI
        var same = Path.Combine(exeDir, "publish-cli.exe");
        if (File.Exists(same)) return same;
        // Check publish/publish-cli relative
        var rel = Path.GetFullPath(Path.Combine(exeDir, "..", "..", "..", "..", "publish-cli", "publish-cli.exe"));
        if (File.Exists(rel)) return rel;
        return string.Empty;
    }

    public string? FindCli() => Found ? _cliPath : null;

    // ── Generic runner ──────────────────────────────────

    public async Task<CliOutput<T>?> RunAsync<T>(string args, string projectPath, int timeoutMs = 30000)
    {
        if (!Found) return Fail<T>("publish-cli.exe not found");

        var fullArgs = $"{args} --json --path \"{projectPath}\"";
        var result = await _ps.RunAsync(_cliPath, fullArgs, timeoutMs: timeoutMs);

        if (!result.Success)
            return Fail<T>(string.IsNullOrEmpty(result.StandardError) ? "execution failed" : result.StandardError);

        if (string.IsNullOrWhiteSpace(result.StandardOutput))
            return Fail<T>("no output from publish-cli");

        try
        {
            return JsonConvert.DeserializeObject<CliOutput<T>>(result.StandardOutput);
        }
        catch (JsonException ex)
        {
            Log.Error(ex, "JSON parse failed: {Output}", result.StandardOutput);
            return Fail<T>($"parse output: {ex.Message}");
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

    public Task<CliOutput<List<ChangeLog>>> GetLogAsync(string projectPath, int limit = 20)
        => RunAsync<List<ChangeLog>>($"log --limit {limit}", projectPath);

    public Task<CliOutput<object>?> ConfigInitAsync(string projectPath, string serverUrl, string projectName, int projectId)
    {
        var args = $"config init --server \"{serverUrl}\" --project \"{projectName}\" --path \"{projectPath}\"";
        if (projectId > 0) args += $" --id {projectId}";
        return RunAsync<object>(args, projectPath);
    }

    public Task<CliOutput<List<ProjectInfo>>?> ProjectListAsync(string serverUrl)
        => RunAsync<List<ProjectInfo>>($"project list --server \"{serverUrl}\"", string.Empty);

    public Task<CliOutput<ProjectInfo>?> ProjectCreateAsync(string serverUrl, string name, string title, bool forceUpdate)
    {
        var args = $"project create --server \"{serverUrl}\" --name \"{name}\" --title \"{title}\"";
        if (forceUpdate) args += " --force-update";
        return RunAsync<ProjectInfo>(args, string.Empty);
    }
}