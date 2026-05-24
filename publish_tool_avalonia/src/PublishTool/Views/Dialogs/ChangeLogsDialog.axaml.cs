using Avalonia.Controls;
using Avalonia.Interactivity;

namespace PublishTool.Views.Dialogs;

public partial class ChangeLogsDialog : Window
{
    public ChangeLogsDialog()
    {
        InitializeComponent();
    }

    private void CloseButton_Click(object? sender, RoutedEventArgs e)
    {
        Close();
    }
}
