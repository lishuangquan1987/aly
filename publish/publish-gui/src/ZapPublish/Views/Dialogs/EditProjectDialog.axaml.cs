using Avalonia.Controls;
using Avalonia.Interactivity;

namespace ZapPublish.Views.Dialogs;

public partial class EditProjectDialog : Window
{
    public EditProjectDialog()
    {
        InitializeComponent();
    }

    private void CancelBtn_Click(object? sender, RoutedEventArgs e)
    {
        Close(null);
    }
}
