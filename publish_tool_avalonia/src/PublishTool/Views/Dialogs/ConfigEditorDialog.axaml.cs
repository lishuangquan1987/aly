using Avalonia.Controls;
using Avalonia.Interactivity;

namespace PublishTool.Views.Dialogs;

public partial class ConfigEditorDialog : Window
{
    public ConfigEditorDialog()
    {
        InitializeComponent();
    }

    private void CancelButton_Click(object? sender, RoutedEventArgs e)
    {
        Close();
    }
}
