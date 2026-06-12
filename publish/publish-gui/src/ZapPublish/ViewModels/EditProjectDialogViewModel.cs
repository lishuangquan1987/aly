using System;
using System.Collections.ObjectModel;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Newtonsoft.Json;
using ZapPublish.Models.Local;
using ZapPublish.Services;
using Serilog;

namespace ZapPublish.ViewModels;

public partial class EditProjectDialogViewModel : ObservableObject
{
    private readonly CliService _cli;
    private string _projectPath = string.Empty;

    [ObservableProperty] private string _displayName = string.Empty;
    [ObservableProperty] private string _serverUrl = string.Empty;
    [ObservableProperty] private string _projectName = string.Empty;
    [ObservableProperty] private ObservableCollection<string> _ignoreFolders = new();
    [ObservableProperty] private ObservableCollection<string> _ignoreFiles = new();
    [ObservableProperty] private bool _isBusy;
    [ObservableProperty] private string _statusMessage = string.Empty;

    // Edit-in-place fields
    [ObservableProperty] private bool _isEditingUrl;
    [ObservableProperty] private string _editUrl = string.Empty;
    [ObservableProperty] private string _newFolderItem = string.Empty;
    [ObservableProperty] private string _newFileItem = string.Empty;

    public event Action<ProjectConfig?>? RequestClose;

    public IAsyncRelayCommand SaveUrlCommand { get; }
    public IAsyncRelayCommand AddFolderCommand { get; }
    public IAsyncRelayCommand RemoveFolderCommand { get; }
    public IAsyncRelayCommand AddFileCommand { get; }
    public IAsyncRelayCommand RemoveFileCommand { get; }
    public IAsyncRelayCommand SaveCommand { get; }

    public EditProjectDialogViewModel(CliService cli)
    {
        _cli = cli;

        SaveUrlCommand = new AsyncRelayCommand(SaveUrlAsync);
        AddFolderCommand = new AsyncRelayCommand<string?>(AddFolderAsync);
        RemoveFolderCommand = new AsyncRelayCommand<string?>(RemoveFolderAsync);
        AddFileCommand = new AsyncRelayCommand<string?>(AddFileAsync);
        RemoveFileCommand = new AsyncRelayCommand<string?>(RemoveFileAsync);
        SaveCommand = new AsyncRelayCommand(SaveAsync);
    }

    /// <summary>
    /// 加载项目配置（由 DialogService 在创建后调用）
    /// </summary>
    public void LoadProject(ProjectConfig project)
    {
        if (project == null) throw new ArgumentNullException(nameof(project));
        _projectPath = project.ProjectPath;
        DisplayName = project.DisplayName;
        LoadCliConfig();
    }

    private void LoadCliConfig()
    {
        var sharedPath = Path.Combine(_projectPath, ".updator", "shared.json");
        if (!File.Exists(sharedPath))
        {
            StatusMessage = ".updator/shared.json 不存在，项目可能未初始化";
            return;
        }

        try
        {
            var json = File.ReadAllText(sharedPath);
            var cfg = JsonConvert.DeserializeObject<SharedConfig>(json);
            if (cfg != null)
            {
                ServerUrl = cfg.ServerUrl ?? "";
                ProjectName = cfg.ProjectName ?? "";
                IgnoreFolders = new ObservableCollection<string>(cfg.IgnoreFolders ?? []);
                IgnoreFiles = new ObservableCollection<string>(cfg.IgnoreFiles ?? []);
            }
        }
        catch (Exception ex)
        {
            Log.Error(ex, "读取 shared.json 失败");
            StatusMessage = $"读取配置失败: {ex.Message}";
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
                StatusMessage = "服务端地址已更新";
            }
            else StatusMessage = $"更新失败: {r?.ErrorMsg}";
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
            }
            else StatusMessage = $"添加失败: {r?.ErrorMsg}";
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
            }
            else StatusMessage = $"移除失败: {r?.ErrorMsg}";
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
            }
            else StatusMessage = $"添加失败: {r?.ErrorMsg}";
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
            }
            else StatusMessage = $"移除失败: {r?.ErrorMsg}";
        }
        finally { IsBusy = false; }
    }

    private Task SaveAsync()
    {
        var result = new ProjectConfig
        {
            DisplayName = DisplayName.Trim(),
            ProjectPath = _projectPath
        };
        RequestClose?.Invoke(result);
        return Task.CompletedTask;
    }
}
