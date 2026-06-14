using System;
using System.Collections.ObjectModel;
using System.Linq;
using System.Threading.Tasks;
using Ursa.Controls;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using ZapPublish.Models.Local;
using ZapPublish.Services;
using Serilog;

namespace ZapPublish.ViewModels;

public partial class MainWindowViewModel : ObservableObject
{
    private readonly CliService _cli;
    private readonly ConfigService _cfg;
    private readonly IDialogService _dialog;

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

    public MainWindowViewModel(CliService cli, ConfigService cfg, IDialogService dialog)
    {
        _cli = cli;
        _cfg = cfg;
        _dialog = dialog;
        IsCliFound = _cli.Found;
        CliPath = _cli.CliPath;
        Log.Information("MainWindowViewModel 初始化: CliFound={Found}, CliPath={Path}", IsCliFound, CliPath);

        AddProjectCommand = new AsyncRelayCommand(AddProjectAsync);
        AddLocalProjectCommand = new AsyncRelayCommand(AddLocalProjectAsync);
        RemoveProjectCommand = new AsyncRelayCommand(RemoveProjectAsync, () => SelectedProject != null);
        EditProjectCommand = new AsyncRelayCommand(EditProjectAsync, () => SelectedProject != null);

        LoadProjects();
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
            tab = new ProjectTabViewModel(value, _cli);
            Tabs.Add(tab);
            _ = tab.RefreshAsync(); // 首次创建时自动刷新
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
            // 为每个项目预创建 tab（加速切换）
            Tabs.Add(new ProjectTabViewModel(c, _cli));
        }
        Log.Information("共加载 {Count} 个项目", Projects.Count);
        if (Projects.Count > 0)
        {
            SelectedProject = Projects[0];
            _ = Tabs[0].RefreshAsync();
        }
    }

    // ── Project CRUD ─────────────────────────────────────

    private async Task AddProjectAsync()
    {
        Log.Information("打开初始化项目对话框");
        var cfg = await _dialog.ShowAddProjectDialogAsync();
        if (cfg != null)
        {
            _cfg.AddProject(cfg);
            Projects.Add(cfg);
            var tab = new ProjectTabViewModel(cfg, _cli);
            Tabs.Add(tab);
            SelectedTab = tab;
            SelectedProject = cfg;
            _ = tab.RefreshAsync();
            await MessageBox.ShowAsync($"项目 \"{cfg.DisplayName}\" 已添加", "成功", MessageBoxIcon.Success);
        }
    }

    private async Task AddLocalProjectAsync()
    {
        Log.Information("打开添加本地项目对话框");
        var cfg = await _dialog.ShowAddLocalProjectDialogAsync();
        if (cfg != null)
        {
            _cfg.AddProject(cfg);
            Projects.Add(cfg);
            var tab = new ProjectTabViewModel(cfg, _cli);
            Tabs.Add(tab);
            SelectedTab = tab;
            SelectedProject = cfg;
            _ = tab.RefreshAsync();
            await MessageBox.ShowAsync($"项目 \"{cfg.DisplayName}\" 已添加", "成功", MessageBoxIcon.Success);
        }
    }

    private async Task RemoveProjectAsync()
    {
        if (SelectedProject == null) return;
        var name = SelectedProject.DisplayName;
        _cfg.RemoveProject(name);

        // 清理对应 tab
        var tab = Tabs.FirstOrDefault(t => t.Project == SelectedProject);
        if (tab != null) Tabs.Remove(tab);

        Projects.Remove(SelectedProject);
        SelectedProject = Projects.FirstOrDefault();
        await MessageBox.ShowAsync($"项目 \"{name}\" 已移除", "成功", MessageBoxIcon.Success);
    }

    private async Task EditProjectAsync()
    {
        if (SelectedProject == null) return;
        var oldProject = SelectedProject;
        var result = await _dialog.ShowEditProjectDialogAsync(SelectedProject);
        if (result != null)
        {
            _cfg.UpdateProject(result);
            var idx = Projects.IndexOf(SelectedProject);
            if (idx >= 0) Projects[idx] = result;

            // 替换旧的 tab
            var oldTab = Tabs.FirstOrDefault(t => t.Project == oldProject);
            if (oldTab != null) Tabs.Remove(oldTab);
            SelectedProject = result; // 触发 OnSelectedProjectChanged 创建新 tab
            await MessageBox.ShowAsync("项目配置已更新", "成功", MessageBoxIcon.Success);
        }
    }
}
