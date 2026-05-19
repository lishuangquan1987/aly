using System;
using Avalonia.Controls;
using PublishTool.ViewModels;

namespace PublishTool.Views.Dialogs;

public partial class AddServerProjectDialog : Window
{
    public AddServerProjectDialog()
    {
        InitializeComponent();
        DataContextChanged += OnDataContextChanged;
    }

    private void OnDataContextChanged(object? sender, EventArgs e)
    {
        if (DataContext is AddServerProjectDialogViewModel vm)
        {
            vm.CloseRequested += () => Close();
        }
    }

    private void CancelButton_Click(object? sender, Avalonia.Interactivity.RoutedEventArgs e)
    {
        Close();
    }
}
