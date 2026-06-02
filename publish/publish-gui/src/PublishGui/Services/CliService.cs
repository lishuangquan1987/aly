using Newtonsoft.Json;
using PublishGui.Models.Cli;
using Serilog;
using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Threading.Tasks;

namespace PublishGui.Services;

public class CliService
{
    private readonly ProcessService _processService;
    private string _cliPath = string.Empty;

    public CliService(ProcessService processService)
    {
        _processService = processService;
    }

    public string CliPath
    {
        get => _cliPath;
        set => _cliPath = value;
    }

    public async Task<string?> FindCliAsync()
    {
        // 1. Check same directory as GUI executable
        var currentDir = AppDomain.CurrentDomain.BaseDirectory;
        var sameDirPath = Path.Combine(currentDir, "publish-cli.exe");
        if (File.Exists(sameDirPath))
        {
            _cliPath = sameDirPath;
            Log.Information("Found publish-cli in same directory: {Path}", _cliPath);
            return _cliPath;
        }

        // 2. Check publish/publish-cli relative to GUI
        var guiDir = new DirectoryInfo(currentDir);
        while (guiDir != null)
        {
            // Check publish/publish-cli/publish-cli.exe
            var publishCliPath = Path.Combine(guiDir.FullName, "publish", "publish-cli", "publish-cli.exe");
            if (File.Exists(publishCliPath))
            {
                _cliPath = publishCliPath;
                Log.Information("Found publish-cli in publish directory: {Path}", _cliPath);
                return _cliPath;
            }

            // Check publish-cli/publish-cli.exe (if running from publish-gui)
            var cliPath2 = Path.Combine(guiDir.FullName, "publish-cli", "publish-cli.exe");
            if (File.Exists(cliPath2))
            {
                _cliPath = cliPath2;
                Log.Information("Found publish-cli in sibling directory: {Path}", _cliPath);
                return _cliPath;
            }

            guiDir = guiDir.Parent;
        }

        // 3. Check PATH environment variable
        var pathEnv = Environment.GetEnvironmentVariable("PATH");
        if (pathEnv != null)
        {
            foreach (var pathDir in pathEnv.Split(';'))
            {
                if (string.IsNullOrWhiteSpace(pathDir)) continue;
                var fullPath = Path.Combine(pathDir.Trim(), "publish-cli.exe");
                if (File.Exists(fullPath))
                {
                    _cliPath = fullPath;
                    Log.Information("Found publish-cli in PATH: {Path}", _cliPath);
                    return _cliPath;
                }
            }
        }

        Log.Warning("publish-cli.exe not found");
        return null;
    }

    public async Task<CliOutput<T>?> RunAsync<T>(string arguments, string? projectPath = null, int timeoutMs = 30000)
    {
        if (string.IsNullOrEmpty(_cliPath))
        {
            return new CliOutput<T>
            {
                IsSuccess = false,
                ErrorMsg = "publish-cli 路径未配置，请在项目设置中配置 publish-cli.exe 路径"
            };
        }

        var args = $"{arguments} --json";
        if (!string.IsNullOrEmpty(projectPath))
        {
            args += $" --path \"{projectPath}\"";
        }

        var result = await _processService.RunAsync(_cliPath, args, timeoutMs: timeoutMs);

        Log.Information("CLI result: ExitCode={Code}, Output={Output}", result.ExitCode, 
            result.StandardOutput.Length > 200 ? result.StandardOutput[..200] + "..." : result.StandardOutput);

        if (!result.Success)
        {
            Log.Error("publish-cli error: {Error}", result.StandardError);
            return new CliOutput<T>
            {
                IsSuccess = false,
                ErrorMsg = string.IsNullOrEmpty(result.StandardError) ? "执行失败" : result.StandardError
            };
        }

        if (string.IsNullOrWhiteSpace(result.StandardOutput))
        {
            return new CliOutput<T>
            {
                IsSuccess = false,
                ErrorMsg = "publish-cli 没有返回任何输出"
            };
        }

        try
        {
            return JsonConvert.DeserializeObject<CliOutput<T>>(result.StandardOutput);
        }
        catch (JsonException ex)
        {
            Log.Error(ex, "Failed to parse CLI output: {Output}", result.StandardOutput);
            return new CliOutput<T>
            {
                IsSuccess = false,
                ErrorMsg = $"解析输出失败: {ex.Message}"
            };
        }
    }

    public async Task<CliOutput<object>?> RunAsync(string arguments, string? projectPath = null, int timeoutMs = 30000)
    {
        return await RunAsync<object>(arguments, projectPath, timeoutMs);
    }

    // Status command
    public async Task<CliOutput<StatusData>?> GetStatusAsync(string projectPath)
    {
        return await RunAsync<StatusData>("status", projectPath);
    }

    // Add --all command
    public async Task<CliOutput<object>?> AddAllAsync(string projectPath)
    {
        return await RunAsync("add --all", projectPath);
    }

    // Add specific files
    public async Task<CliOutput<object>?> AddFilesAsync(string projectPath, IEnumerable<string> files)
    {
        var fileList = string.Join(" ", files.Select(f => $"\"{f}\""));
        return await RunAsync($"add {fileList}", projectPath);
    }

    // Reset --all command
    public async Task<CliOutput<object>?> ResetAllAsync(string projectPath)
    {
        return await RunAsync("reset --all", projectPath);
    }

    // Reset specific files
    public async Task<CliOutput<object>?> ResetFilesAsync(string projectPath, IEnumerable<string> files)
    {
        var fileList = string.Join(" ", files.Select(f => $"\"{f}\""));
        return await RunAsync($"reset {fileList}", projectPath);
    }

    // Publish command
    public async Task<CliOutput<object>?> PublishAsync(string projectPath, string version, string message)
    {
        return await RunAsync($"publish --version \"{version}\" --message \"{message}\"", projectPath, timeoutMs: 120000);
    }

    // Push command
    public async Task<CliOutput<object>?> PushAsync(string projectPath, string version, string message)
    {
        return await RunAsync($"push --version \"{version}\" --message \"{message}\"", projectPath, timeoutMs: 120000);
    }

    // Log command
    public async Task<CliOutput<List<ChangeLog>>> GetLogAsync(string projectPath, int limit = 20)
    {
        return await RunAsync<List<ChangeLog>>($"log --limit {limit}", projectPath);
    }

    // Config init command
    public async Task<CliOutput<object>?> ConfigInitAsync(string projectPath, string serverUrl, string projectName, int projectId = 0)
    {
        var args = $"config init --server \"{serverUrl}\" --project \"{projectName}\" --path \"{projectPath}\"";
        if (projectId > 0)
        {
            args += $" --id {projectId}";
        }
        return await RunAsync(args, projectPath);
    }

    // Config set command
    public async Task<CliOutput<object>?> ConfigSetAsync(string projectPath, string key, string value)
    {
        return await RunAsync($"config set {key} \"{value}\"", projectPath);
    }

    // Config list command
    public async Task<CliOutput<object>?> ConfigListAsync(string projectPath)
    {
        return await RunAsync("config list", projectPath);
    }

    // Project list command
    public async Task<CliOutput<List<ProjectInfo>>> ProjectListAsync(string serverUrl)
    {
        return await RunAsync<List<ProjectInfo>>($"project list --server \"{serverUrl}\"");
    }

    // Project create command
    public async Task<CliOutput<ProjectInfo>> ProjectCreateAsync(string serverUrl, string name, string title, bool forceUpdate = false)
    {
        var args = $"project create --server \"{serverUrl}\" --name \"{name}\" --title \"{title}\"";
        if (forceUpdate) args += " --force-update";
        return await RunAsync<ProjectInfo>(args);
    }

    // Project delete command
    public async Task<CliOutput<object>?> ProjectDeleteAsync(string serverUrl, int projectId)
    {
        return await RunAsync($"project delete --server \"{serverUrl}\" --id {projectId}");
    }

    // Server info command
    public async Task<CliOutput<List<ServerOsInfo>>> ServerInfoAsync(string serverUrl, int projectId)
    {
        return await RunAsync<List<ServerOsInfo>>($"server info --server \"{serverUrl}\" --id {projectId}");
    }
}
