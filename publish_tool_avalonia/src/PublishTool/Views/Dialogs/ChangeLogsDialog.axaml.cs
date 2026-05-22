using System.Collections.ObjectModel;
using Avalonia.Controls;
using Avalonia.Interactivity;
using PublishTool.Models;

namespace PublishTool.Views.Dialogs;

public partial class ChangeLogsDialog : Window
{
    public ChangeLogsDialog()
    {
        InitializeComponent();
    }

    public ChangeLogsDialog(ObservableCollection<ProjectChangeLog> changeLogs) : this()
    {
        LogsList.ItemsSource = changeLogs;
    }

    private void CloseButton_Click(object? sender, RoutedEventArgs e)
    {
        Close();
    }
}
