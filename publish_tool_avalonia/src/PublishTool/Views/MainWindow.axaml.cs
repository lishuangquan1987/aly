using System;
using System.Linq;
using System.Threading.Tasks;
using Avalonia.Controls;
using Avalonia.Interactivity;
using Avalonia.Platform.Storage;
using PublishTool.Models.Local;
using PublishTool.Services;
using PublishTool.ViewModels;
using PublishTool.Views.Controls;
using PublishTool.Views.Dialogs;
using Microsoft.Extensions.DependencyInjection;

namespace PublishTool.Views;

public partial class MainWindow : Window
{
    public MainWindow()
    {
        InitializeComponent();
    }

    private async void ConnectServer_Click(object? sender, RoutedEventArgs e)
    {
        if (DataContext is not MainWindowViewModel vm || string.IsNullOrWhiteSpace(vm.ServerUrl))
            return;

        try
        {
            var projectService = App.Services.GetRequiredService<ProjectService>();
            var projects = await projectService.GetAllProjectsAsync(vm.ServerUrl);
        }
        catch (Exception)
        {
            // 连接失败，静默处理
        }
    }

    public async Task ShowAddServerProjectDialog()
    {
        if (DataContext is not MainWindowViewModel vm) return;

        var projectService = App.Services.GetRequiredService<ProjectService>();
        var dialogVm = new AddServerProjectDialogViewModel(projectService);
        dialogVm.ServerUrl = vm.ServerUrl;
        var dialog = new AddServerProjectDialog { DataContext = dialogVm };
        await dialog.ShowDialog(this);
    }

    public async Task ShowAddClientProjectDialog()
    {
        if (DataContext is not MainWindowViewModel vm) return;

        var projectService = App.Services.GetRequiredService<ProjectService>();
        var configService = App.Services.GetRequiredService<ConfigService>();
        var dialogVm = new AddProjectDialogViewModel(projectService, configService);
        dialogVm.ServerUrl = vm.ServerUrl;
        var dialog = new AddProjectDialog { DataContext = dialogVm };
        await dialog.ShowDialog(this);
    }

    public async Task ShowProjectSettingsDialog(ProjectPageViewModel pageVm)
    {
        if (DataContext is not MainWindowViewModel vm) return;

        var configService = App.Services.GetRequiredService<ConfigService>();
        var dialogVm = new ProjectSettingsDialogViewModel(configService, pageVm.Config);
        var dialog = new ProjectSettingsDialog { DataContext = dialogVm };
        await dialog.ShowDialog(this);

        pageVm.RefreshConfig();
    }

    public async Task ShowConfigEditorDialog(ProjectPageViewModel pageVm)
    {
        if (DataContext is not MainWindowViewModel vm) return;

        var configService = App.Services.GetRequiredService<ConfigService>();
        var dialogVm = new ConfigEditorDialogViewModel(pageVm.Config, configService);
        var dialog = new ConfigEditorDialog { DataContext = dialogVm };
        await dialog.ShowDialog(this);
    }
}