using Avalonia.Controls.Notifications;
using Ursa.Controls;

namespace ZapPublish.Services;

public class ToastService : IToastService
{
    private readonly IToastManager _toastManager;

    public ToastService(IToastManager toastManager)
    {
        _toastManager = toastManager;
    }

    public void ShowSuccess(string message) =>
        _toastManager.Show(new Toast(message, NotificationType.Success, TimeSpan.FromSeconds(3)));

    public void ShowError(string message) =>
        _toastManager.Show(new Toast(message, NotificationType.Error, TimeSpan.FromSeconds(5)));

    public void ShowWarning(string message) =>
        _toastManager.Show(new Toast(message, NotificationType.Warning, TimeSpan.FromSeconds(4)));

    public void ShowInfo(string message) =>
        _toastManager.Show(new Toast(message, NotificationType.Information, TimeSpan.FromSeconds(3)));
}
