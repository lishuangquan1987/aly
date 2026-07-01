using System;
using System.Collections.ObjectModel;
using System.Linq;
using System.Threading.Tasks;
using Avalonia.Controls;
using Avalonia.Controls.ApplicationLifetimes;
using Avalonia.Platform.Storage;
using Ursa.Controls;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Microsoft.Extensions.DependencyInjection;
using AlyPublish.Models.Cli;
using AlyPublish.Models.Local;
using AlyPublish.Services;
using AlyPublish.Views.Dialogs;
using Serilog;

namespace AlyPublish.ViewModels;

public partial class MainWindowViewModel : ObservableObject
{
    private readonly CliService _cli;
    private readonly ConfigService _cfg;
    private readonly IServiceProvider _sp;

    [ObservableProperty] private ObservableCollection<ProjectConfig> _projects = new();
    [NotifyCanExecuteChangedFor(nameof(RemoveProjectCommand))]
    [NotifyCanExecuteChangedFor(nameof(EditProjectCommand))]
    [ObservableProperty] private ProjectConfig? _selectedProject;
    [ObservableProperty] private ObservableCollection<ProjectTabViewModel> _tabs = new();
    [ObservableProperty] private ProjectTabViewModel? _selectedTab;
    [ObservableProperty] private bool _isCliFound;
    [ObservableProperty] private string _cliPath = string.Empty;
    [ObservableProperty] private string _statusMessage = "就绪";

    public IAsyncRelayCommand AddProjectCommand { get; }
    public IAsyncRelayCommand AddLocalProjectCommand { get; }
    public IAsyncRelayCommand RemoveProjectCommand { get; }
    public IAsyncRelayCommand EditProjectCommand { get; }
    public IRelayCommand<ProjectTabViewModel> CloseTabCommand { get; }

    public MainWindowViewModel(CliService cli, ConfigService cfg, IServiceProvider sp)
    {
        _cli = cli;
        _cfg = cfg;
        _sp = sp;
        IsCliFound = _cli.Found;
        CliPath = _cli.CliPath;
        Log.Information("MainWindowViewModel 初始化: CliFound={Found}, CliPath={Path}", IsCliFound, CliPath);

        AddProjectCommand = new AsyncRelayCommand(AddProjectAsync);
        AddLocalProjectCommand = new AsyncRelayCommand(AddLocalProjectAsync);
        RemoveProjectCommand = new AsyncRelayCommand(RemoveProjectAsync, () => SelectedProject != null);
        EditProjectCommand = new AsyncRelayCommand(EditProjectAsync, () => SelectedProject != null);
        CloseTabCommand = new RelayCommand<ProjectTabViewModel>(CloseTab);

        LoadProjects();
    }

    private Window? GetOwner()
    {
        var lifetime = Avalonia.Application.Current?.ApplicationLifetime
            as IClassicDesktopStyleApplicationLifetime;
        var owner = lifetime?.MainWindow;
        if (owner == null)
            Log.Warning("GetOwner 返回 null，对话框无法显示");
        return owner;
    }

    private async Task<ProjectConfig?> ShowAddProjectDialogAsync()
    {
        var owner = GetOwner();
        if (owner == null) return null;

        var dialog = new AddProjectDialog();
        var browseFolderAsync = new Func<Task<string?>>(async () =>
        {
            var result = await dialog.StorageProvider.OpenFolderPickerAsync(
                new FolderPickerOpenOptions { Title = "选择项目文件夹" });
            return result.Count > 0 ? result[0].Path.LocalPath : null;
        });

        var vm = _sp.GetRequiredService<AddProjectDialogViewModel>();
        vm.BrowseFolderAsync = browseFolderAsync;
        vm.ShowCreateProjectDialogAsync = async (serverUrl, ignoreFolders, ignoreFiles) =>
            await ShowCreateProjectDialogAsync(dialog, serverUrl, ignoreFolders, ignoreFiles);

        dialog.DataContext = vm;
        vm.RequestClose += result => dialog.Close(result);

        Log.Information("打开添加项目对话框");
        return await dialog.ShowDialog<ProjectConfig?>(owner);
    }

    private async Task<ProjectConfig?> ShowAddLocalProjectDialogAsync()
    {
        var owner = GetOwner();
        if (owner == null) return null;

        var dialog = new AddLocalProjectDialog();
        var browseFolderAsync = new Func<Task<string?>>(async () =>
        {
            var result = await dialog.StorageProvider.OpenFolderPickerAsync(
                new FolderPickerOpenOptions { Title = "选择已初始化的项目文件夹" });
            return result.Count > 0 ? result[0].Path.LocalPath : null;
        });

        var vm = _sp.GetRequiredService<AddLocalProjectDialogViewModel>();
        vm.BrowseFolderAsync = browseFolderAsync;

        dialog.DataContext = vm;
        vm.RequestClose += result => dialog.Close(result);

        Log.Information("打开添加本地项目对话框");
        return await dialog.ShowDialog<ProjectConfig?>(owner);
    }

    private async Task<ProjectInfo?> ShowCreateProjectDialogAsync(Window owner, string serverUrl, string ignoreFolders = "", string ignoreFiles = "")
    {
        var dialog = new CreateProjectDialog();
        var vm = _sp.GetRequiredService<CreateProjectDialogViewModel>();
        vm.ServerUrl = serverUrl;
        vm.IgnoreFolders = ignoreFolders;
        vm.IgnoreFiles = ignoreFiles;

        dialog.DataContext = vm;
        vm.RequestClose += result => dialog.Close(result);

        Log.Information("打开创建项目对话框: ServerUrl={Url}", serverUrl);
        return await dialog.ShowDialog<ProjectInfo?>(owner);
    }

    private async Task<ProjectConfig?> ShowEditProjectDialogAsync(ProjectConfig project)
    {
        var owner = GetOwner();
        if (owner == null) return null;

        var dialog = new EditProjectDialog();
        var vm = _sp.GetRequiredService<EditProjectDialogViewModel>();
        vm.LoadProject(project);

        dialog.DataContext = vm;
        vm.RequestClose += result => dialog.Close(result);

        Log.Information("打开编辑项目对话框: DisplayName={Name}", project.DisplayName);
        return await dialog.ShowDialog<ProjectConfig?>(owner);
    }

    private ProjectTabViewModel CreateTab(ProjectConfig project)
    {
        var tab = new ProjectTabViewModel(project, _cli);
        tab.EditProjectAsync = async (cfg) =>
        {
            var result = await ShowEditProjectDialogAsync(cfg);
            if (result != null)
            {
                if (!_cfg.UpdateProject(result))
                    await MessageBox.ShowAsync("项目配置保存到磁盘失败", "警告", MessageBoxIcon.Warning);
                var idx = Projects.IndexOf(project);
                if (idx >= 0) Projects[idx] = result;
                // 替换旧的 tab 引用（OnSelectedProjectChanged 会重建 tab）
                var oldTab = Tabs.FirstOrDefault(t => t.Project == project);
                if (oldTab != null)
                {
                    SelectedTab = null;
                    Tabs.Remove(oldTab);
                    oldTab.Dispose();
                }
                SelectedProject = result;
            }
            return result;
        };
        return tab;
    }

    partial void OnSelectedProjectChanged(ProjectConfig? value)
    {
        Log.Information("切换项目: {Name}", value?.DisplayName ?? "(无)");
        RemoveProjectCommand.NotifyCanExecuteChanged();
        EditProjectCommand.NotifyCanExecuteChanged();

        if (value == null)
        {
            SelectedTab = null;
            return;
        }

        // 查找已有的 tab，没有则创建
        var tab = Tabs.FirstOrDefault(t => t.Project == value);
        if (tab == null)
        {
            tab = CreateTab(value);
            Tabs.Add(tab);
            FireAndForget(tab.RefreshAsync(), "RefreshAsync");
        }
        SelectedTab = tab;
    }

    private void LoadProjects()
    {
        Log.Information("加载本地项目配置");
        Projects.Clear();
        foreach (var c in _cfg.LoadProjects())
        {
            Log.Debug("加载项目: DisplayName={Name}, Path={Path}", c.DisplayName, c.ProjectPath);
            Projects.Add(c);
        }
        Log.Information("共加载 {Count} 个项目", Projects.Count);
        if (Projects.Count > 0)
            SelectedProject = Projects[0]; // 触发 OnSelectedProjectChanged 按需创建 tab
    }

    private void FireAndForget(Task task, string context)
    {
        _ = task.ContinueWith(t =>
        {
            if (t.IsFaulted && t.Exception != null)
                Log.Error(t.Exception, "{Context}", context);
        }, TaskContinuationOptions.OnlyOnFaulted);
    }

    // ── Project CRUD ─────────────────────────────────────

    private async Task AddProjectAsync()
    {
        Log.Information("打开初始化项目对话框");
        var cfg = await ShowAddProjectDialogAsync();
        if (cfg != null)
        {
            if (!_cfg.AddProject(cfg))
                await MessageBox.ShowAsync("项目配置保存到磁盘失败", "警告", MessageBoxIcon.Warning);
            Projects.Add(cfg);
            var tab = CreateTab(cfg);
            Tabs.Add(tab);
            SelectedProject = cfg;
            SelectedTab = tab;
            FireAndForget(tab.RefreshAsync(), "RefreshAsync");
            await MessageBox.ShowAsync($"项目 \"{cfg.DisplayName}\" 已添加", "成功", MessageBoxIcon.Success);
        }
    }

    private async Task AddLocalProjectAsync()
    {
        Log.Information("打开添加本地项目对话框");
        var cfg = await ShowAddLocalProjectDialogAsync();
        if (cfg != null)
        {
            if (!_cfg.AddProject(cfg))
                await MessageBox.ShowAsync("项目配置保存到磁盘失败", "警告", MessageBoxIcon.Warning);
            Projects.Add(cfg);
            var tab = CreateTab(cfg);
            Tabs.Add(tab);
            SelectedProject = cfg;
            SelectedTab = tab;
            FireAndForget(tab.RefreshAsync(), "RefreshAsync");
            await MessageBox.ShowAsync($"项目 \"{cfg.DisplayName}\" 已添加", "成功", MessageBoxIcon.Success);
        }
    }

    private async Task RemoveProjectAsync()
    {
        if (SelectedProject == null) return;
        var name = SelectedProject.DisplayName;
        var path = SelectedProject.ProjectPath;
        if (!_cfg.RemoveProject(path))
            await MessageBox.ShowAsync("项目配置保存到磁盘失败", "警告", MessageBoxIcon.Warning);

        // 清理对应 tab
        var tab = Tabs.FirstOrDefault(t => t.Project == SelectedProject);
        if (tab != null)
        {
            SelectedTab = null;
            Tabs.Remove(tab);
        }

        Projects.Remove(SelectedProject);
        SelectedProject = Projects.FirstOrDefault();
        await MessageBox.ShowAsync($"项目 \"{name}\" 已移除", "成功", MessageBoxIcon.Success);
    }

    private void CloseTab(ProjectTabViewModel? tab)
    {
        if (tab == null) return;
        var idx = Tabs.IndexOf(tab);
        if (idx < 0) return;

        Tabs.RemoveAt(idx);
        tab.Dispose();
        // 如果关闭的是当前选中的 tab，切换到相邻 tab（sidebar 选中项不变）
        if (SelectedTab == tab)
        {
            if (Tabs.Count > 0)
                SelectedTab = Tabs[Math.Min(idx, Tabs.Count - 1)];
            else
                SelectedTab = null;
        }
    }

    private async Task EditProjectAsync()
    {
        if (SelectedProject == null) return;
        var oldProject = SelectedProject;
        var result = await ShowEditProjectDialogAsync(SelectedProject);
        if (result != null)
        {
            if (!_cfg.UpdateProject(result))
                await MessageBox.ShowAsync("项目配置保存到磁盘失败", "警告", MessageBoxIcon.Warning);
            var idx = Projects.IndexOf(SelectedProject);
            if (idx >= 0) Projects[idx] = result;

            // 替换旧的 tab
            var oldTab = Tabs.FirstOrDefault(t => t.Project == oldProject);
            if (oldTab != null) { Tabs.Remove(oldTab); oldTab.Dispose(); }
            SelectedProject = result; // 触发 OnSelectedProjectChanged 创建新 tab
            await MessageBox.ShowAsync("项目配置已更新", "成功", MessageBoxIcon.Success);
        }
    }
}
