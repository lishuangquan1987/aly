using System;
using System.Linq;
using System.Threading.Tasks;
using Avalonia.Controls;
using Avalonia.Interactivity;
using Avalonia.Platform.Storage;
using Microsoft.Extensions.DependencyInjection;
using PublishTool.Models;
using PublishTool.Models.Local;
using PublishTool.Services;
using PublishTool.ViewModels;
using PublishTool.Views.Controls;
using PublishTool.Views.Dialogs;

namespace PublishTool.Views;

public partial class MainWindow : Window
{
    private MainWindowViewModel? _currentVm;

    public MainWindow()
    {
        InitializeComponent();
        DataContextChanged += OnDataContextChanged;
    }

    private void OnDataContextChanged(object? sender, EventArgs e)
    {
        if (_currentVm != null)
        {
            _currentVm.AddServerProjectRequested -= ShowAddServerProjectDialog;
            _currentVm.AddClientProjectRequested -= ShowAddClientProjectDialog;
            _currentVm.DeleteProjectRequested -= ShowDeleteConfirmDialog;
            _currentVm.OpenTabs.CollectionChanged -= OnTabsChanged;
            _currentVm.PropertyChanged -= OnVmPropertyChanged;
        }

        if (DataContext is MainWindowViewModel vm)
        {
            _currentVm = vm;
            vm.AddServerProjectRequested += ShowAddServerProjectDialog;
            vm.AddClientProjectRequested += ShowAddClientProjectDialog;
            vm.DeleteProjectRequested += ShowDeleteConfirmDialog;

            vm.OpenTabs.CollectionChanged += OnTabsChanged;
            vm.PropertyChanged += OnVmPropertyChanged;
            UpdateEmptyContentOverlay();
        }
    }

    private void OnTabsChanged(object? sender, System.Collections.Specialized.NotifyCollectionChangedEventArgs e)
    {
        UpdateEmptyContentOverlay();
    }

    private void OnVmPropertyChanged(object? sender, System.ComponentModel.PropertyChangedEventArgs e)
    {
        if (e.PropertyName == nameof(MainWindowViewModel.SelectedTab))
            UpdateEmptyContentOverlay();
    }

    private void UpdateEmptyContentOverlay()
    {
        if (DataContext is MainWindowViewModel vm)
        {
            var overlay = this.FindControl<Panel>("EmptyContentOverlay");
            if (overlay != null)
                overlay.IsVisible = vm.SelectedTab == null;
        }
    }

    public async Task ShowAddServerProjectDialog()
    {
        var projectService = App.Services.GetRequiredService<ProjectService>();
        var dialogVm = new AddServerProjectDialogViewModel(projectService);
        var dialog = new AddServerProjectDialog { DataContext = dialogVm };
        await dialog.ShowDialog(this);
    }

    public async Task ShowAddClientProjectDialog()
    {
        var projectService = App.Services.GetRequiredService<ProjectService>();
        var configService = App.Services.GetRequiredService<ConfigService>();
        var dialogVm = new AddProjectDialogViewModel(projectService, configService);
        var dialog = new AddProjectDialog { DataContext = dialogVm };
        await dialog.ShowDialog(this);
    }

    public async Task ShowProjectSettingsDialog(ProjectPageViewModel pageVm)
    {
        var configService = App.Services.GetRequiredService<ConfigService>();
        var dialogVm = new ProjectSettingsDialogViewModel(configService, pageVm.Config);
        var dialog = new ProjectSettingsDialog { DataContext = dialogVm };
        await dialog.ShowDialog(this);

        pageVm.RefreshConfig();
    }

    public async Task ShowConfigEditorDialog(ProjectPageViewModel pageVm)
    {
        var configService = App.Services.GetRequiredService<ConfigService>();
        var dialogVm = new ConfigEditorDialogViewModel(pageVm.Config, configService);
        var dialog = new ConfigEditorDialog { DataContext = dialogVm };
        await dialog.ShowDialog(this);
    }

    public async Task ShowChangeLogsDialog(ProjectPageViewModel pageVm)
    {
        var dialog = new ChangeLogsDialog { DataContext = new ChangeLogsDialogViewModel(pageVm.ChangeLogs) };
        await dialog.ShowDialog(this);
    }

    public async Task ShowDeleteConfirmDialog()
    {
        if (DataContext is MainWindowViewModel vm && vm.SelectedProject != null)
        {
            var project = vm.SelectedProject;
            var dialog = new DeleteConfirmDialog { DataContext = new DeleteConfirmDialogViewModel(project.Title) };
            var result = await dialog.ShowDialog<bool>(this);
            if (result)
            {
                vm.ConfirmRemoveProject(project);
            }
        }
    }
}
