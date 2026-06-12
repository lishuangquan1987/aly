using Avalonia.Controls;
using Ursa.Controls;
using ZapPublish.Services;

namespace ZapPublish.Views;

public partial class MainWindow : Window
{
    public WindowToastManager ToastManager { get; }

    public MainWindow()
    {
        InitializeComponent();
        ToastManager = new WindowToastManager(this);
        DataContextChanged += OnDataContextChanged;
    }

    private void OnDataContextChanged(object? sender, EventArgs e)
    {
        if (DataContext is ViewModels.MainWindowViewModel vm)
        {
            vm.ToastService = new ToastService(ToastManager);
            DataContextChanged -= OnDataContextChanged;
        }
    }
}
