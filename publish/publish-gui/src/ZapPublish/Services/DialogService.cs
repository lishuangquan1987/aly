using System;
using System.Threading.Tasks;
using Avalonia.Controls;
using Avalonia.Platform.Storage;
using Microsoft.Extensions.DependencyInjection;
using ZapPublish.Models.Cli;
using ZapPublish.Models.Local;
using ZapPublish.ViewModels;
using ZapPublish.Views.Dialogs;
using Serilog;

namespace ZapPublish.Services;

public class DialogService : IDialogService
{
    private readonly IServiceProvider _sp;

    public DialogService(IServiceProvider sp)
    {
        _sp = sp;
    }

    private Window? GetOwner()
    {
        var lifetime = Avalonia.Application.Current?.ApplicationLifetime
            as Avalonia.Controls.ApplicationLifetimes.IClassicDesktopStyleApplicationLifetime;
        return lifetime?.MainWindow;
    }

    public async Task<ProjectConfig?> ShowAddProjectDialogAsync()
    {
        var owner = GetOwner();
        if (owner == null)
        {
            Log.Warning("无法打开添加项目对话框: MainWindow 不可用");
            return null;
        }

        var cli = _sp.GetRequiredService<CliService>();
        var dialog = new AddProjectDialog();

        // browseFolderAsync 捕获 dialog 实例以访问 StorageProvider
        var browseFolderAsync = new Func<Task<string?>>(async () =>
        {
            var result = await dialog.StorageProvider.OpenFolderPickerAsync(
                new FolderPickerOpenOptions { Title = "选择项目文件夹" });
            return result.Count > 0 ? result[0].Path.LocalPath : null;
        });

        var vm = new AddProjectDialogViewModel(cli, browseFolderAsync);

        // 注入子对话框委托
        vm.ShowCreateProjectDialogAsync = async (serverUrl) =>
            await ShowCreateProjectDialogAsync(serverUrl);

        dialog.DataContext = vm;
        vm.RequestClose += result => dialog.Close(result);

        Log.Information("打开添加项目对话框");
        return await dialog.ShowDialog<ProjectConfig?>(owner);
    }

    public async Task<ProjectInfo?> ShowCreateProjectDialogAsync(string serverUrl)
    {
        var owner = GetOwner();
        if (owner == null)
        {
            Log.Warning("无法打开创建项目对话框: MainWindow 不可用");
            return null;
        }

        var cli = _sp.GetRequiredService<CliService>();
        var dialog = new CreateProjectDialog();
        var vm = new CreateProjectDialogViewModel(cli) { ServerUrl = serverUrl };

        dialog.DataContext = vm;
        vm.RequestClose += result => dialog.Close(result);

        Log.Information("打开创建项目对话框: ServerUrl={Url}", serverUrl);
        return await dialog.ShowDialog<ProjectInfo?>(owner);
    }

    public async Task<ProjectConfig?> ShowEditProjectDialogAsync(ProjectConfig project)
    {
        var owner = GetOwner();
        if (owner == null)
        {
            Log.Warning("无法打开编辑项目对话框: MainWindow 不可用");
            return null;
        }

        var cli = _sp.GetRequiredService<CliService>();
        var dialog = new EditProjectDialog();
        var vm = new EditProjectDialogViewModel(cli, project);

        dialog.DataContext = vm;
        vm.RequestClose += result => dialog.Close(result);

        Log.Information("打开编辑项目对话框: DisplayName={Name}", project.DisplayName);
        return await dialog.ShowDialog<ProjectConfig?>(owner);
    }
}
