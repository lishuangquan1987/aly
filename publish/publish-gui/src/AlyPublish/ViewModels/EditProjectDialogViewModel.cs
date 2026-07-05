using System;
using System.Collections.ObjectModel;

using System.IO;
using System.Linq;
using System.Threading.Tasks;
using Avalonia.Controls;
using Avalonia.Platform.Storage;
using Avalonia.Threading;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Newtonsoft.Json;
using Ursa.Controls;
using AlyPublish.Models.Local;
using AlyPublish.Services;
using Serilog;

namespace AlyPublish.ViewModels;

public partial class EditProjectDialogViewModel : ObservableObject
{
    private readonly CliService _cli;
    private string _projectPath = string.Empty;

    [ObservableProperty] private string _displayName = string.Empty;
    [ObservableProperty] private string _serverUrl = string.Empty;
    [ObservableProperty] private string _projectName = string.Empty;
    [ObservableProperty] private ObservableCollection<string> _ignoreFolders = new();
    [ObservableProperty] private ObservableCollection<string> _ignoreFiles = new();
    [ObservableProperty] private ObservableCollection<string> _unCopyFolders = new();
    [ObservableProperty] private ObservableCollection<string> _unCopyFiles = new();
    [ObservableProperty] private bool _isBusy;
    [ObservableProperty] private string _statusMessage = string.Empty;
    [ObservableProperty] private string _mainExePath = string.Empty;

    [ObservableProperty] private bool _isEditingUrl;
    [ObservableProperty] private string _editUrl = string.Empty;
    [ObservableProperty] private string _newFolderItem = string.Empty;
    [ObservableProperty] private string _newFileItem = string.Empty;
    [ObservableProperty] private string _newUnCopyFolderItem = string.Empty;
    [ObservableProperty] private string _newUnCopyFileItem = string.Empty;

    public event Action<ProjectConfig?>? RequestClose;

    public IAsyncRelayCommand SaveUrlCommand { get; }
    public IRelayCommand StartEditUrlCommand { get; }
    public IAsyncRelayCommand AddFolderCommand { get; }
    public IAsyncRelayCommand RemoveFolderCommand { get; }
    public IAsyncRelayCommand AddFileCommand { get; }
    public IAsyncRelayCommand RemoveFileCommand { get; }
    public IAsyncRelayCommand AddUnCopyFolderCommand { get; }
    public IAsyncRelayCommand RemoveUnCopyFolderCommand { get; }
    public IAsyncRelayCommand AddUnCopyFileCommand { get; }
    public IAsyncRelayCommand RemoveUnCopyFileCommand { get; }
    public IAsyncRelayCommand BrowseExeCommand { get; }
    public IRelayCommand CancelCommand { get; }
    public IAsyncRelayCommand SaveCommand { get; }

    public EditProjectDialogViewModel(CliService cli)
    {
        _cli = cli;

        SaveUrlCommand = new AsyncRelayCommand(SaveUrlAsync);
        AddFolderCommand = new AsyncRelayCommand<string?>(AddFolderAsync);
        RemoveFolderCommand = new AsyncRelayCommand<string?>(RemoveFolderAsync);
        AddFileCommand = new AsyncRelayCommand<string?>(AddFileAsync);
        RemoveFileCommand = new AsyncRelayCommand<string?>(RemoveFileAsync);
        AddUnCopyFolderCommand = new AsyncRelayCommand<string?>(AddUnCopyFolderAsync);
        RemoveUnCopyFolderCommand = new AsyncRelayCommand<string?>(RemoveUnCopyFolderAsync);
        AddUnCopyFileCommand = new AsyncRelayCommand<string?>(AddUnCopyFileAsync);
        RemoveUnCopyFileCommand = new AsyncRelayCommand<string?>(RemoveUnCopyFileAsync);
        BrowseExeCommand = new AsyncRelayCommand(BrowseExeAsync);
        CancelCommand = new RelayCommand(() => RequestClose?.Invoke(null));
        SaveCommand = new AsyncRelayCommand(SaveAsync);
        StartEditUrlCommand = new RelayCommand(StartEditUrl);
    }

    private void StartEditUrl()
    {
        EditUrl = ServerUrl;
        IsEditingUrl = true;
    }

    public void LoadProject(ProjectConfig project)
    {
        if (project == null) throw new ArgumentNullException(nameof(project));
        _projectPath = project.ProjectPath;
        DisplayName = project.DisplayName;
        MainExePath = project.MainExePath ?? string.Empty;
        _ = LoadCliConfigAsync().ContinueWith(t =>
        {
            if (t.IsFaulted) Log.Error(t.Exception, "LoadCliConfig failed");
        }, TaskScheduler.Default);
    }

    private async Task LoadCliConfigAsync()
    {
        var sharedPath = Path.Combine(_projectPath, ".updator", "shared.json");
        if (!File.Exists(sharedPath))
        {
            await Dispatcher.UIThread.InvokeAsync(() =>
                MessageBox.ShowAsync(".updator/shared.json 不存在，项目可能未初始化", "错误", MessageBoxIcon.Error));
            return;
        }

        try
        {
            var json = File.ReadAllText(sharedPath);
            var cfg = JsonConvert.DeserializeObject<SharedConfig>(json);
            if (cfg != null)
            {
                // 在 UI 线程上设置属性，避免跨线程访问
                await Dispatcher.UIThread.InvokeAsync(() =>
                {
                    ServerUrl = cfg.ServerUrl ?? "";
                    ProjectName = cfg.ProjectName ?? "";
                    IgnoreFolders = new ObservableCollection<string>(cfg.IgnoreFolders ?? []);
                    IgnoreFiles = new ObservableCollection<string>(cfg.IgnoreFiles ?? []);
                    UnCopyFolders = new ObservableCollection<string>(cfg.UnCopyFolders ?? []);
                    UnCopyFiles = new ObservableCollection<string>(cfg.UnCopyFiles ?? []);
                });
            }
        }
        catch (Exception ex)
        {
            Log.Error(ex, "读取 shared.json 失败");
            await Dispatcher.UIThread.InvokeAsync(() =>
                MessageBox.ShowAsync($"读取配置失败: {ex.Message}", "错误", MessageBoxIcon.Error));
        }
    }

    private async Task SaveUrlAsync()
    {
        if (string.IsNullOrWhiteSpace(EditUrl)) return;
        IsBusy = true;
        try
        {
            var r = await _cli.ConfigSetAsync(_projectPath, "server.url", EditUrl.Trim());
            if (r?.IsSuccess == true)
            {
                ServerUrl = EditUrl.Trim();
                IsEditingUrl = false;
                await MessageBox.ShowAsync("服务端地址已更新", "成功", MessageBoxIcon.Success);
            }
            else
            {
                await MessageBox.ShowAsync($"更新失败: {r?.ErrorMsg}", "错误", MessageBoxIcon.Error);
            }
        }
        catch (Exception ex)
        {
            Log.Error(ex, "更新服务端地址失败: {Url}", EditUrl);
            await MessageBox.ShowAsync($"操作异常: {ex.Message}", "错误", MessageBoxIcon.Error);
        }
        finally { IsBusy = false; }
    }

    private async Task AddFolderAsync(string? item)
    {
        if (string.IsNullOrWhiteSpace(item)) return;
        IsBusy = true;
        try
        {
            var r = await _cli.ConfigSetArrayAddAsync(_projectPath, "ignore.folders", item.Trim());
            if (r?.IsSuccess == true)
            {
                IgnoreFolders.Add(item.Trim());
                NewFolderItem = string.Empty;
                StatusMessage = "已添加";
                await SyncIgnoreToServerAsync();
            }
            else
            {
                await MessageBox.ShowAsync($"添加失败: {r?.ErrorMsg}", "错误", MessageBoxIcon.Error);
            }
        }
        catch (Exception ex)
        {
            Log.Error(ex, "添加忽略文件夹失败: {Item}", item);
            await MessageBox.ShowAsync($"操作异常: {ex.Message}", "错误", MessageBoxIcon.Error);
        }
        finally { IsBusy = false; }
    }

    private async Task RemoveFolderAsync(string? item)
    {
        if (string.IsNullOrWhiteSpace(item)) return;
        IsBusy = true;
        try
        {
            var r = await _cli.ConfigSetArrayRemoveAsync(_projectPath, "ignore.folders", item);
            if (r?.IsSuccess == true)
            {
                IgnoreFolders.Remove(item);
                StatusMessage = "已移除";
                await SyncIgnoreToServerAsync();
            }
            else
            {
                await MessageBox.ShowAsync($"移除失败: {r?.ErrorMsg}", "错误", MessageBoxIcon.Error);
            }
        }
        catch (Exception ex)
        {
            Log.Error(ex, "移除忽略文件夹失败: {Item}", item);
            await MessageBox.ShowAsync($"操作异常: {ex.Message}", "错误", MessageBoxIcon.Error);
        }
        finally { IsBusy = false; }
    }

    private async Task AddFileAsync(string? item)
    {
        if (string.IsNullOrWhiteSpace(item)) return;
        IsBusy = true;
        try
        {
            var r = await _cli.ConfigSetArrayAddAsync(_projectPath, "ignore.files", item.Trim());
            if (r?.IsSuccess == true)
            {
                IgnoreFiles.Add(item.Trim());
                NewFileItem = string.Empty;
                StatusMessage = "已添加";
                await SyncIgnoreToServerAsync();
            }
            else
            {
                await MessageBox.ShowAsync($"添加失败: {r?.ErrorMsg}", "错误", MessageBoxIcon.Error);
            }
        }
        catch (Exception ex)
        {
            Log.Error(ex, "添加忽略文件失败: {Item}", item);
            await MessageBox.ShowAsync($"操作异常: {ex.Message}", "错误", MessageBoxIcon.Error);
        }
        finally { IsBusy = false; }
    }

    private async Task RemoveFileAsync(string? item)
    {
        if (string.IsNullOrWhiteSpace(item)) return;
        IsBusy = true;
        try
        {
            var r = await _cli.ConfigSetArrayRemoveAsync(_projectPath, "ignore.files", item);
            if (r?.IsSuccess == true)
            {
                IgnoreFiles.Remove(item);
                StatusMessage = "已移除";
                await SyncIgnoreToServerAsync();
            }
            else
            {
                await MessageBox.ShowAsync($"移除失败: {r?.ErrorMsg}", "错误", MessageBoxIcon.Error);
            }
        }
        catch (Exception ex)
        {
            Log.Error(ex, "移除忽略文件失败: {Item}", item);
            await MessageBox.ShowAsync($"操作异常: {ex.Message}", "错误", MessageBoxIcon.Error);
        }
        finally { IsBusy = false; }
    }

    // ── UnCopy Folders ────────────────────────────────────

    private async Task AddUnCopyFolderAsync(string? item)
    {
        if (string.IsNullOrWhiteSpace(item)) return;
        IsBusy = true;
        try
        {
            var r = await _cli.ConfigSetArrayAddAsync(_projectPath, "un_copy.folders", item.Trim());
            if (r?.IsSuccess == true)
            {
                UnCopyFolders.Add(item.Trim());
                NewUnCopyFolderItem = string.Empty;
                StatusMessage = "已添加（不复制文件夹）";
            }
            else
            {
                await MessageBox.ShowAsync($"添加失败: {r?.ErrorMsg}", "错误", MessageBoxIcon.Error);
            }
        }
        catch (Exception ex)
        {
            Log.Error(ex, "添加不复制文件夹失败: {Item}", item);
            await MessageBox.ShowAsync($"操作异常: {ex.Message}", "错误", MessageBoxIcon.Error);
        }
        finally { IsBusy = false; }
    }

    private async Task RemoveUnCopyFolderAsync(string? item)
    {
        if (string.IsNullOrWhiteSpace(item)) return;
        IsBusy = true;
        try
        {
            var r = await _cli.ConfigSetArrayRemoveAsync(_projectPath, "un_copy.folders", item);
            if (r?.IsSuccess == true)
            {
                UnCopyFolders.Remove(item);
                StatusMessage = "已移除（不复制文件夹）";
            }
            else
            {
                await MessageBox.ShowAsync($"移除失败: {r?.ErrorMsg}", "错误", MessageBoxIcon.Error);
            }
        }
        catch (Exception ex)
        {
            Log.Error(ex, "移除不复制文件夹失败: {Item}", item);
            await MessageBox.ShowAsync($"操作异常: {ex.Message}", "错误", MessageBoxIcon.Error);
        }
        finally { IsBusy = false; }
    }

    // ── UnCopy Files ──────────────────────────────────────

    private async Task AddUnCopyFileAsync(string? item)
    {
        if (string.IsNullOrWhiteSpace(item)) return;
        IsBusy = true;
        try
        {
            var r = await _cli.ConfigSetArrayAddAsync(_projectPath, "un_copy.files", item.Trim());
            if (r?.IsSuccess == true)
            {
                UnCopyFiles.Add(item.Trim());
                NewUnCopyFileItem = string.Empty;
                StatusMessage = "已添加（不复制文件）";
            }
            else
            {
                await MessageBox.ShowAsync($"添加失败: {r?.ErrorMsg}", "错误", MessageBoxIcon.Error);
            }
        }
        catch (Exception ex)
        {
            Log.Error(ex, "添加不复制文件失败: {Item}", item);
            await MessageBox.ShowAsync($"操作异常: {ex.Message}", "错误", MessageBoxIcon.Error);
        }
        finally { IsBusy = false; }
    }

    private async Task RemoveUnCopyFileAsync(string? item)
    {
        if (string.IsNullOrWhiteSpace(item)) return;
        IsBusy = true;
        try
        {
            var r = await _cli.ConfigSetArrayRemoveAsync(_projectPath, "un_copy.files", item);
            if (r?.IsSuccess == true)
            {
                UnCopyFiles.Remove(item);
                StatusMessage = "已移除（不复制文件）";
            }
            else
            {
                await MessageBox.ShowAsync($"移除失败: {r?.ErrorMsg}", "错误", MessageBoxIcon.Error);
            }
        }
        catch (Exception ex)
        {
            Log.Error(ex, "移除不复制文件失败: {Item}", item);
            await MessageBox.ShowAsync($"操作异常: {ex.Message}", "错误", MessageBoxIcon.Error);
        }
        finally { IsBusy = false; }
    }

    // ── Browse MainExe ────────────────────────────────────

    private async Task BrowseExeAsync()
    {
        try
        {
            var topLevel = TopLevel.GetTopLevel(Avalonia.Application.Current?.ApplicationLifetime is Avalonia.Controls.ApplicationLifetimes.IClassicDesktopStyleApplicationLifetime desktop
                ? desktop.MainWindow : null);
            if (topLevel == null) return;

            var files = await topLevel.StorageProvider.OpenFilePickerAsync(new FilePickerOpenOptions
            {
                Title = "选择主程序 (.exe)",
                AllowMultiple = false,
                FileTypeFilter = [new FilePickerFileType("可执行文件") { Patterns = ["*.exe"] }]
            });

            if (files.Count > 0)
            {
                var selectedPath = files[0].Path.LocalPath;
                // 转换为相对路径（相对于 ProjectPath）
                if (!string.IsNullOrWhiteSpace(_projectPath))
                {
                    var projectDir = Path.GetFullPath(_projectPath).TrimEnd(Path.DirectorySeparatorChar, Path.AltDirectorySeparatorChar);
                    var selectedFull = Path.GetFullPath(selectedPath);
                    if (selectedFull.StartsWith(projectDir + Path.DirectorySeparatorChar, StringComparison.OrdinalIgnoreCase) ||
                        selectedFull.StartsWith(projectDir + Path.AltDirectorySeparatorChar, StringComparison.OrdinalIgnoreCase))
                    {
                        MainExePath = selectedFull.Substring(projectDir.Length + 1);
                    }
                    else
                    {
                        // 不在项目目录内，使用绝对路径
                        MainExePath = selectedFull;
                        await MessageBox.ShowAsync("所选文件不在项目目录内，将使用绝对路径", "提示", MessageBoxIcon.Warning);
                    }
                }
                else
                {
                    MainExePath = selectedPath;
                }
            }
        }
        catch (Exception ex)
        {
            Log.Error(ex, "选择主程序失败");
            await MessageBox.ShowAsync($"选择失败: {ex.Message}", "错误", MessageBoxIcon.Error);
        }
    }

    /// <summary>将当前的 ignore 设置同步到服务端（保留服务端原有 title，不覆盖）</summary>
    private async Task SyncIgnoreToServerAsync()
    {
        if (string.IsNullOrWhiteSpace(ServerUrl) || string.IsNullOrWhiteSpace(ProjectName))
            return;
        try
        {
            // 先获取服务端项目信息，避免将 title 覆盖为 projectName
            var listResult = await _cli.ProjectListAsync(ServerUrl);
            if (listResult?.IsSuccess != true || listResult.Data == null)
            {
                Log.Warning("同步 ignore 前获取项目列表失败: {Error}", listResult?.ErrorMsg);
                StatusMessage = "本地已更新，但服务端同步失败";
                return;
            }
            var serverProject = listResult.Data.FirstOrDefault(p => p.Name == ProjectName);
            if (serverProject == null)
            {
                Log.Warning("服务端未找到项目: {Name}", ProjectName);
                StatusMessage = "本地已更新，但服务端未找到该项目";
                return;
            }
            var r = await _cli.ProjectUpdateAsync(ServerUrl, ProjectName, serverProject.Title,
                false, IgnoreFolders.ToList(), IgnoreFiles.ToList());
            if (r?.IsSuccess != true)
            {
                Log.Warning("同步 ignore 到服务端失败: {Error}", r?.ErrorMsg);
                StatusMessage = "本地已更新，但服务端同步失败";
            }
        }
        catch (Exception ex)
        {
            Log.Warning(ex, "同步 ignore 到服务端异常");
            StatusMessage = "本地已更新，但服务端同步异常";
        }
    }

    private Task SaveAsync()
    {
        var result = new ProjectConfig
        {
            DisplayName = DisplayName.Trim(),
            ProjectPath = _projectPath,
            MainExePath = MainExePath?.Trim() ?? string.Empty
        };
        RequestClose?.Invoke(result);
        return Task.CompletedTask;
    }
}
