using Avalonia.Controls.Notifications;
using Serilog;
using Ursa.Controls;

namespace AlyPublish.Services;

/// <summary>Non-blocking user feedback. The toast host is attached by the main window.</summary>
public interface INotificationService
{
    void Success(string message);

    void Info(string message);

    void Warning(string message);

    void ShowError(string message);
}

public sealed class NotificationService : INotificationService
{
    private static readonly TimeSpan ToastDuration = TimeSpan.FromSeconds(4);

    private WindowToastManager? _manager;

    public void Attach(Avalonia.Controls.TopLevel topLevel)
    {
        _manager = new WindowToastManager(topLevel) { MaxItems = 3 };
    }

    public void Detach()
    {
        _manager?.Uninstall();
        _manager = null;
    }

    public void Success(string message) => Show(message, NotificationType.Success);

    public void Info(string message) => Show(message, NotificationType.Information);

    public void Warning(string message) => Show(message, NotificationType.Warning);

    public void ShowError(string message) => Show(message, NotificationType.Error);

    private void Show(string message, NotificationType type)
    {
        if (_manager is null)
        {
            Log.Debug("通知宿主尚未就绪: {Message}", message);
            return;
        }

        _manager.Show(new Toast(message, type)
        {
            ShowIcon = true,
            ShowClose = true,
            Expiration = ToastDuration
        });
    }
}
