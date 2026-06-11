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

    private void EditUrlBtn_Click(object? sender, RoutedEventArgs e)
    {
        var vm = DataContext as ViewModels.EditProjectDialogViewModel;
        if (vm == null) return;
        vm.EditUrl = vm.ServerUrl;
        vm.IsEditingUrl = true;
    }
}
