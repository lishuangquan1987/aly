using Newtonsoft.Json;
using PublishGui.Models.Cli;
using Serilog;
using System;
using System.Diagnostics;
using System.IO;
using System.Threading.Tasks;

namespace PublishGui.Services;

public class CliService
{
    private string _cliPath = string.Empty;

    public string CliPath
    {
        get => _cliPath;
        set => _cliPath = value;
    }

    public async Task<string?> FindCliAsync()
    {
        // Check same directory first
        var currentDir = AppDomain.CurrentDomain.BaseDirectory;
        var sameDirPath = Path.Combine(currentDir, "publish-cli.exe");
        if (File.Exists(sameDirPath))
        {
            _cliPath = sameDirPath;
            return _cliPath;
        }

        // Check PATH
        var pathEnv = Environment.GetEnvironmentVariable("PATH");
        if (pathEnv != null)
        {
            foreach (var dir in pathEnv.Split(';'))
            {
                var fullPath = Path.Combine(dir, "publish-cli.exe");
                if (File.Exists(fullPath))
                {
                    _cliPath = fullPath;
                    return _cliPath;
                }
            }
        }

        return null;
    }

    public async Task<CliOutput<T>?> RunAsync<T>(string arguments, string? projectPath = null)
    {
        if (string.IsNullOrEmpty(_cliPath))
        {
            throw new InvalidOperationException("publish-cli path not configured");
        }

        var args = $"{arguments} --json";
        if (!string.IsNullOrEmpty(projectPath))
        {
            args += $" --path \"{projectPath}\"";
        }

        Log.Information("Running: {CliPath} {Args}", _cliPath, args);

        var psi = new ProcessStartInfo
        {
            FileName = _cliPath,
            Arguments = args,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            UseShellExecute = false,
            CreateNoWindow = true
        };

        using var process = Process.Start(psi);
        if (process == null)
        {
            throw new Exception("Failed to start publish-cli process");
        }

        var output = await process.StandardOutput.ReadToEndAsync();
        var error = await process.StandardError.ReadToEndAsync();
        await process.WaitForExitAsync();

        Log.Information("Exit code: {Code}, Output: {Output}", process.ExitCode, output);

        if (process.ExitCode != 0)
        {
            Log.Error("publish-cli error: {Error}", error);
            return new CliOutput<T>
            {
                IsSuccess = false,
                ErrorMsg = error
            };
        }

        return JsonConvert.DeserializeObject<CliOutput<T>>(output);
    }

    public async Task<CliOutput<object>?> RunAsync(string arguments, string? projectPath = null)
    {
        return await RunAsync<object>(arguments, projectPath);
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
        return await RunAsync($"publish --version \"{version}\" --message \"{message}\"", projectPath);
    }

    // Push command
    public async Task<CliOutput<object>?> PushAsync(string projectPath, string version, string message)
    {
        return await RunAsync($"push --version \"{version}\" --message \"{message}\"", projectPath);
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
