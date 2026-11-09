using AlyPublish.ViewModels;
using Avalonia.Controls;
using Avalonia.Interactivity;

namespace AlyPublish.Views.Dialogs;

public partial class EditProjectDialog : Window
{
    public EditProjectDialog()
    {
        InitializeComponent();
        Closing += OnClosing;
    }

    private async void OnBrowseMainExe(object? sender, RoutedEventArgs e)
    {
        var path = await StoragePicker.PickExecutableAsync(this, "选择主程序");
        if (path is not null && DataContext is EditProjectDialogViewModel viewModel)
        {
            viewModel.MainExePath = path;
        }
    }

    private void OnClosing(object? sender, WindowClosingEventArgs e)
    {
        if (DataContext is ViewModelBase { IsBusy: true })
        {
            e.Cancel = true;
        }
    }
}
