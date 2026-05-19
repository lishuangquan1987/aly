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
        DataContextChanged += OnDataContextChanged;
    }

    private void OnDataContextChanged(object? sender, EventArgs e)
    {
        if (DataContext is MainWindowViewModel vm)
        {
            vm.AddServerProjectRequested += ShowAddServerProjectDialog;
            vm.AddClientProjectRequested += ShowAddClientProjectDialog;
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
}
