using AlyPublish.Localization;
using AlyPublish.Services;
using CommunityToolkit.Mvvm.ComponentModel;
using Serilog;

namespace AlyPublish.ViewModels;

/// <summary>
/// Shared UI state and the single error-handling boundary. Commands stay short:
/// they validate, call a service, and let this base class handle busy state and failures.
/// </summary>
public abstract partial class ViewModelBase : ObservableObject
{
    protected ViewModelBase(IDialogService dialogs, INotificationService notifications)
    {
        Dialogs = dialogs;
        Notify = notifications;
    }

    protected IDialogService Dialogs { get; }

    protected INotificationService Notify { get; }

    [ObservableProperty]
    private bool _isBusy;

    [ObservableProperty]
    private string _statusMessage = LanguageManager.T(LanguageManager.Instance.Ready);

    /// <summary>The view subscribes to close the dialog with a result.</summary>
    public event Action<object?>? RequestClose;

    partial void OnIsBusyChanged(bool value) => OnBusyChanged();

    /// <summary>Called after <see cref="IsBusy"/> changes so subclasses can refresh command state.</summary>
    protected virtual void OnBusyChanged()
    {
    }

    protected void Close(object? result) => RequestClose?.Invoke(result);

    protected Task<bool> RunAsync(
        string operation,
        Action work,
        bool showErrorDialog = true,
        CancellationToken cancellationToken = default)
        => RunAsync(
            operation,
            _ =>
            {
                work();
                return Task.CompletedTask;
            },
            showErrorDialog,
            cancellationToken);

    /// <returns><see langword="true"/> only when the operation completed successfully.</returns>
    protected async Task<bool> RunAsync(
        string operation,
        Func<CancellationToken, Task> work,
        bool showErrorDialog = true,
        CancellationToken cancellationToken = default)
    {
        if (IsBusy)
        {
            return false;
        }

        IsBusy = true;
        try
        {
            await work(cancellationToken);
            return true;
        }
        catch (OperationCanceledException)
        {
            StatusMessage = LanguageManager.F(LanguageManager.Instance.Msg_Cancelled, operation);
            Log.Information("{Operation} 已取消", operation);
            return false;
        }
        catch (CliException ex)
        {
            await ReportFailureAsync(operation, ex.Message, showErrorDialog);
            return false;
        }
        catch (ProjectConfigException ex)
        {
            await ReportFailureAsync(operation, ex.Message, showErrorDialog);
            return false;
        }
        catch (Exception ex)
        {
            Log.Error(ex, "{Operation} 发生未预期异常", operation);
            await ReportFailureAsync(
                operation,
                LanguageManager.F(LanguageManager.Instance.Msg_UnexpectedFailure, operation, ex.Message),
                showErrorDialog);
            return false;
        }
        finally
        {
            IsBusy = false;
        }
    }

    private async Task ReportFailureAsync(string operation, string message, bool showErrorDialog)
    {
        StatusMessage = LanguageManager.F(LanguageManager.Instance.Msg_Failed, operation, message);
        Log.Warning("{Operation} 失败: {Message}", operation, message);

        if (showErrorDialog)
        {
            await Dialogs.ErrorAsync(LanguageManager.F(LanguageManager.Instance.Msg_FailedTitle, operation), message);
        }
        else
        {
            Notify.ShowError(message);
        }
    }
}
