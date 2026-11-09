using AlyPublish.ViewModels;
using Avalonia.Controls;

namespace AlyPublish.Views.Dialogs;

public partial class CreateProjectDialog : Window
{
    public CreateProjectDialog()
    {
        InitializeComponent();
        Closing += OnClosing;
    }

    private void OnClosing(object? sender, WindowClosingEventArgs e)
    {
        if (DataContext is ViewModelBase { IsBusy: true })
        {
            e.Cancel = true;
        }
    }
}
