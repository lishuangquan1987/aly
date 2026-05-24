using Avalonia.Controls;
using Avalonia.Interactivity;

namespace PublishTool.Views.Dialogs;

public partial class DeleteConfirmDialog : Window
{
    public DeleteConfirmDialog()
    {
        InitializeComponent();
    }

    private void CancelButton_Click(object? sender, RoutedEventArgs e)
    {
        Close(false);
    }

    private void ConfirmButton_Click(object? sender, RoutedEventArgs e)
    {
        Close(true);
    }
}
