using System.Collections.ObjectModel;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using PublishTool.Models;
using PublishTool.Models.Local;
using PublishTool.Services;
using Serilog;

namespace PublishTool.ViewModels;

public partial class ProjectPageViewModel : ObservableObject, IDisposable
{
    private readonly ProjectService _projectService;
    private readonly FileService _fileService;
    private readonly LocalFileService _localFileService;
    private readonly ProcessService _processService;
    private CancellationTokenSource? _cts;
    private bool _disposed;

    [ObservableProperty]
    private ProjectConfig _config;

    [ObservableProperty]
    private ServerOsInfoDto? _serverOsInfo;

    [ObservableProperty]
    private string _serverVersion = string.Empty;

    [ObservableProperty]
    private string _latestChangeLog = string.Empty;

    [ObservableProperty]
    private ObservableCollection<LocalFileItem> _localFiles = new();

    [ObservableProperty]
    private ObservableCollection<UploadFileItem> _uploadFiles = new();

    [ObservableProperty]
    private string _newVersion = string.Empty;

    [ObservableProperty]
    private string _changeLogText = string.Empty;

    [ObservableProperty]
    private bool _isUploading;

    [ObservableProperty]
    private string _statusMessage = string.Empty;

    [ObservableProperty]
    private bool _isRefreshEnabled = true;

    [ObservableProperty]
    private int _uploadProgress;

    [ObservableProperty]
    private bool _appendToLatestVersion;

    [ObservableProperty]
    private bool _autoRefreshAfterPush;

    public ProjectPageViewModel(
        ProjectConfig config,
        ProjectService projectService,
        FileService fileService,
        LocalFileService localFileService,
        ProcessService processService)
    {
        _config = config;
        _projectService = projectService;
        _fileService = fileService;
        _localFileService = localFileService;
        _processService = processService;
    }

    public void RefreshConfig()
    {
        OnPropertyChanged(nameof(Config));
    }

    [RelayCommand]
    private async Task RefreshStatus()
    {
        IsRefreshEnabled = false;
        StatusMessage = "正在刷新服务端状态...";
        Log.Information("开始刷新项目状态: {Name}", Config.Name);
        try
        {
            var osInfo = await _projectService.GetOsInfoAsync(Config.ServerUrl, Config.ServerId);
            if (osInfo != null)
            {
                ServerOsInfo = osInfo;
            }

            var logs = await _projectService.GetChangeLogsAsync(Config.ServerUrl, Config.ServerId);
            if (logs.Count > 0)
            {
                var latest = logs[0];
                ServerVersion = latest.Version;
                LatestChangeLog = string.Join("\n", latest.Logs);
            }

            var files = await _fileService.GetAllFilesAsync(Config.ServerUrl, Config.ServerId);
            var modifiedFiles = _localFileService.GetModifiedFiles(
                Config.LocalPath, files, Config.IgnoreFolders, Config.IgnoreFiles);
            LocalFiles = new ObservableCollection<LocalFileItem>(modifiedFiles);

            StatusMessage = "刷新完成";
            Log.Information("刷新完成，文件数: {Count}", modifiedFiles.Count);
        }
        catch (Exception ex)
        {
            StatusMessage = $"刷新失败: {ex.Message}";
            Log.Error(ex, "刷新项目状态失败: {Name}", Config.Name);
        }
        finally
        {
            IsRefreshEnabled = true;
        }
    }

    [RelayCommand]
    private void ScanLocalFiles()
    {
        if (string.IsNullOrEmpty(Config.LocalPath)) return;
        var files = _localFileService.ScanDirectory(
            Config.LocalPath, Config.IgnoreFolders, Config.IgnoreFiles);
        LocalFiles = new ObservableCollection<LocalFileItem>(files);
        StatusMessage = $"扫描到 {files.Count} 个本地文件";
    }

    [RelayCommand]
    private void AddToUploadQueue()
    {
        var checkedFiles = LocalFiles.Where(f => f.IsChecked).ToList();
        foreach (var file in checkedFiles)
        {
            var existing = UploadFiles.FirstOrDefault(f => f.RelativePath == file.RelativePath);
            if (existing == null)
            {
                UploadFiles.Add(new UploadFileItem
                {
                    FileName = file.FileName,
                    LocalPath = file.AbsolutePath,
                    RelativePath = file.RelativePath,
                    LastModified = file.LastModified,
                    Status = UploadStatus.Pending
                });
            }
        }
        StatusMessage = $"已添加 {checkedFiles.Count} 个文件到上传队列";
    }

    [RelayCommand]
    private void RemoveFromQueue()
    {
        var checkedItems = UploadFiles.Where(f => f.IsChecked).ToList();
        foreach (var item in checkedItems)
        {
            UploadFiles.Remove(item);
        }
        StatusMessage = $"已移除 {checkedItems.Count} 个文件";
    }

    [RelayCommand]
    private void ClearQueue()
    {
        UploadFiles.Clear();
        StatusMessage = "上传队列已清空";
    }

    [RelayCommand]
    private async Task DownloadSelected()
    {
        var checkedFiles = LocalFiles.Where(f => f.IsChecked).ToList();
        if (checkedFiles.Count == 0)
        {
            StatusMessage = "请先选择要下载的文件";
            return;
        }
        StatusMessage = $"正在下载 {checkedFiles.Count} 个文件...";
        _cts = new CancellationTokenSource();
        try
        {
            var serverFiles = await _fileService.GetAllFilesAsync(Config.ServerUrl, Config.ServerId);
            foreach (var item in checkedFiles)
            {
                _cts.Token.ThrowIfCancellationRequested();
                var serverFile = serverFiles.FirstOrDefault(f =>
                    f.FileRelativePath == item.RelativePath);
                if (serverFile == null) continue;

                var localPath = Path.Combine(Config.LocalPath, item.RelativePath);
                var dir = Path.GetDirectoryName(localPath);
                if (dir != null) Directory.CreateDirectory(dir);

                await using var stream = await _fileService.DownloadFileAsync(
                    Config.ServerUrl, serverFile.FileAbsolutePath);
                await using var fs = File.Create(localPath);
                await stream.CopyToAsync(fs);
            }
            StatusMessage = "下载完成";
            ScanLocalFiles();
        }
        catch (OperationCanceledException)
        {
            StatusMessage = "下载已取消";
        }
        catch (Exception ex)
        {
            StatusMessage = $"下载失败: {ex.Message}";
        }
        finally
        {
            _cts = null;
        }
    }

    [RelayCommand]
    private async Task PushAll()
    {
        if (UploadFiles.Count == 0) return;
        IsUploading = true;
        _cts = new CancellationTokenSource();
        try
        {
            foreach (var item in UploadFiles)
            {
                _cts.Token.ThrowIfCancellationRequested();
                item.Status = UploadStatus.Uploading;
                StatusMessage = $"正在上传: {item.FileName}";

                await using var stream = File.OpenRead(item.LocalPath);
                await _fileService.UploadFileAsync(
                    Config.ServerUrl, Config.Name, item.RelativePath, stream);

                item.Status = UploadStatus.Done;
            }
            StatusMessage = "所有文件上传完成";
        }
        catch (OperationCanceledException)
        {
            StatusMessage = "上传已取消";
        }
        catch (Exception ex)
        {
            StatusMessage = $"上传失败: {ex.Message}";
            foreach (var item in UploadFiles.Where(f => f.Status == UploadStatus.Uploading))
            {
                item.Status = UploadStatus.Failed;
            }
        }
        finally
        {
            IsUploading = false;
            _cts = null;
        }
    }

    [RelayCommand]
    private async Task DownloadAll()
    {
        StatusMessage = "正在获取服务端文件列表...";
        _cts = new CancellationTokenSource();
        try
        {
            var files = await _fileService.GetAllFilesAsync(Config.ServerUrl, Config.ServerId);
            _cts.Token.ThrowIfCancellationRequested();
            StatusMessage = $"正在下载 {files.Count} 个文件...";
            foreach (var file in files)
            {
                _cts.Token.ThrowIfCancellationRequested();
                var localPath = Path.Combine(Config.LocalPath, file.FileRelativePath);
                var dir = Path.GetDirectoryName(localPath);
                if (dir != null) Directory.CreateDirectory(dir);

                await using var stream = await _fileService.DownloadFileAsync(
                    Config.ServerUrl, file.FileAbsolutePath);
                await using var fs = File.Create(localPath);
                await stream.CopyToAsync(fs);
            }
            StatusMessage = "全部下载完成";
            ScanLocalFiles();
        }
        catch (OperationCanceledException)
        {
            StatusMessage = "下载已取消";
        }
        catch (Exception ex)
        {
            StatusMessage = $"下载失败: {ex.Message}";
        }
        finally
        {
            _cts = null;
        }
    }

    [RelayCommand]
    private async Task PullUpdate()
    {
        StatusMessage = "正在对比文件差异...";
        _cts = new CancellationTokenSource();
        try
        {
            var serverFiles = await _fileService.GetAllFilesAsync(Config.ServerUrl, Config.ServerId);
            _cts.Token.ThrowIfCancellationRequested();
            var modifiedFiles = _localFileService.GetModifiedFiles(
                Config.LocalPath, serverFiles, Config.IgnoreFolders, Config.IgnoreFiles);
            var toDownload = modifiedFiles.Where(f => f.IsModified).ToList();

            StatusMessage = $"发现 {toDownload.Count} 个差异文件，开始下载...";
            foreach (var item in toDownload)
            {
                _cts.Token.ThrowIfCancellationRequested();
                var serverFile = serverFiles.First(f =>
                    f.FileRelativePath == item.RelativePath);
                var localPath = Path.Combine(Config.LocalPath, item.RelativePath);
                var dir = Path.GetDirectoryName(localPath);
                if (dir != null) Directory.CreateDirectory(dir);

                await using var stream = await _fileService.DownloadFileAsync(
                    Config.ServerUrl, serverFile.FileAbsolutePath);
                await using var fs = File.Create(localPath);
                await stream.CopyToAsync(fs);
            }
            StatusMessage = "增量拉取完成";
            ScanLocalFiles();
        }
        catch (OperationCanceledException)
        {
            StatusMessage = "拉取已取消";
        }
        catch (Exception ex)
        {
            StatusMessage = $"拉取失败: {ex.Message}";
        }
        finally
        {
            _cts = null;
        }
    }

    [RelayCommand]
    private void Stop()
    {
        _cts?.Cancel();
        StatusMessage = "操作已停止";
    }

    [RelayCommand]
    private void AutoGenerateVersion()
    {
        NewVersion = DateTime.Now.ToString("yyyyMMdd-HHmm");
    }

    [RelayCommand]
    private void Build()
    {
        StatusMessage = "打包功能开发中...";
    }

    [RelayCommand]
    private void StartDefault()
    {
        if (!string.IsNullOrEmpty(Config.ExePath))
        {
            _processService.StartProcess(Config.ExePath);
        }
        else
        {
            StatusMessage = "未配置 EXE 启动路径";
        }
    }

    [RelayCommand]
    private void StartCustom()
    {
        if (!string.IsNullOrEmpty(Config.ExePath))
        {
            _processService.StartProcess(Config.ExePath);
        }
        else
        {
            StatusMessage = "未配置 EXE 启动路径";
        }
    }

    [RelayCommand]
    private void ViewLogs()
    {
        var logDir = Path.Combine(
            Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
            "PublishTool", "logs");
        if (Directory.Exists(logDir))
        {
            _processService.OpenFolder(logDir);
        }
        else
        {
            StatusMessage = "日志目录不存在";
        }
    }

    [RelayCommand]
    private void OpenExplorer()
    {
        _processService.OpenFolder(Config.LocalPath);
    }

    [RelayCommand]
    private void Close()
    {
        Dispose();
    }

    public void Dispose()
    {
        if (_disposed) return;
        _disposed = true;
        _cts?.Cancel();
        _cts?.Dispose();
    }
}