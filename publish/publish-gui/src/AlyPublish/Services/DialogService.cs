using AlyPublish.Models.Cli;
using AlyPublish.Models.Local;
using AlyPublish.ViewModels;
using AlyPublish.Views.Dialogs;
using Avalonia;
using Avalonia.Controls;
using Avalonia.Controls.ApplicationLifetimes;
using Microsoft.Extensions.DependencyInjection;
using Serilog;
using Ursa.Controls;

namespace AlyPublish.Services;

public sealed class DialogService : IDialogService
{
    private readonly IServiceProvider _services;

    public DialogService(IServiceProvider services)
    {
        _services = services;
    }

    public async Task<ProjectConfig?> ShowAddProjectAsync()
    {
        var owner = GetOwner();
        if (owner is null)
        {
            return null;
        }

        var dialog = new AddProjectDialog();
        var viewModel = _services.GetRequiredService<AddProjectDialogViewModel>();
        viewModel.Initialize();
        return await ShowDialogAsync<ProjectConfig, AddProjectDialogViewModel>(dialog, viewModel, owner);
    }

    public async Task<ProjectConfig?> ShowAddLocalProjectAsync()
    {
        var owner = GetOwner();
        if (owner is null)
        {
            return null;
        }

        var dialog = new AddLocalProjectDialog();
        var viewModel = _services.GetRequiredService<AddLocalProjectDialogViewModel>();
        viewModel.Initialize();
        return await ShowDialogAsync<ProjectConfig, AddLocalProjectDialogViewModel>(dialog, viewModel, owner);
    }

    public async Task<ProjectConfig?> ShowEditProjectAsync(ProjectConfig project)
    {
        var owner = GetOwner();
        if (owner is null)
        {
            return null;
        }

        var dialog = new EditProjectDialog();
        var viewModel = _services.GetRequiredService<EditProjectDialogViewModel>();
        viewModel.Load(project);
        return await ShowDialogAsync<ProjectConfig, EditProjectDialogViewModel>(dialog, viewModel, owner);
    }

    public async Task<ProjectInfo?> ShowCreateProjectAsync(CreateServerProjectRequest request)
    {
        var owner = GetOwner();
        if (owner is null)
        {
            return null;
        }

        var dialog = new CreateProjectDialog();
        var viewModel = _services.GetRequiredService<CreateProjectDialogViewModel>();
        viewModel.Apply(request);
        return await ShowDialogAsync<ProjectInfo, CreateProjectDialogViewModel>(dialog, viewModel, owner);
    }

    public async Task<bool> ConfirmAsync(string title, string message)
    {
        var owner = GetOwner();
        if (owner is null)
        {
            return false;
        }

        var result = await MessageBox.ShowAsync(
            owner,
            message,
            title,
            MessageBoxIcon.Warning,
            MessageBoxButton.OKCancel);
        return result == MessageBoxResult.OK;
    }

    public async Task ErrorAsync(string title, string message)
    {
        var owner = GetOwner();
        if (owner is null)
        {
            return;
        }

        await MessageBox.ShowAsync(owner, message, title, MessageBoxIcon.Error, MessageBoxButton.OK);
    }

    private static async Task<TResult?> ShowDialogAsync<TResult, TViewModel>(
        Window dialog,
        TViewModel viewModel,
        Window owner)
        where TViewModel : ViewModelBase
    {
        dialog.DataContext = viewModel;
        viewModel.RequestClose += result => dialog.Close(result);
        return await dialog.ShowDialog<TResult?>(owner);
    }

    private static Window? GetOwner()
    {
        var lifetime = Application.Current?.ApplicationLifetime as IClassicDesktopStyleApplicationLifetime;
        var active = lifetime?.Windows.LastOrDefault(window => window.IsVisible && window.IsActive);
        if (active is not null)
        {
            return active;
        }

        var owner = lifetime?.Windows.LastOrDefault(window => window.IsVisible) ?? lifetime?.MainWindow;
        if (owner is null)
        {
            Log.Warning("主窗口尚未创建，无法显示对话框");
        }

        return owner;
    }
}
