using AlyPublish.Services;
using Avalonia;
using Avalonia.Controls;
using Microsoft.Extensions.DependencyInjection;

namespace AlyPublish.Views;

public partial class MainWindow : Window
{
    private NotificationService? _notifications;

    public MainWindow()
    {
        InitializeComponent();
    }

    protected override void OnAttachedToVisualTree(VisualTreeAttachmentEventArgs e)
    {
        base.OnAttachedToVisualTree(e);
        _notifications = (Application.Current as App)?.Services
            .GetService<INotificationService>() as NotificationService;
        _notifications?.Attach(this);
    }

    protected override void OnDetachedFromVisualTree(VisualTreeAttachmentEventArgs e)
    {
        _notifications?.Detach();
        _notifications = null;
        base.OnDetachedFromVisualTree(e);
    }
}
