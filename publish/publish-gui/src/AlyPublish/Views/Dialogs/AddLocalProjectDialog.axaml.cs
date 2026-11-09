using AlyPublish.ViewModels;
using Avalonia.Controls;
using Avalonia.Input;
using Avalonia.Interactivity;

namespace AlyPublish.Views.Dialogs;

public partial class AddLocalProjectDialog : Window
{
    public AddLocalProjectDialog()
    {
        InitializeComponent();
        DragDrop.SetAllowDrop(this, true);
        AddHandler(DragDrop.DragOverEvent, OnDragOver);
        AddHandler(DragDrop.DropEvent, OnDrop);
        Closing += OnClosing;
    }

    private static void OnDragOver(object? sender, DragEventArgs e)
        => e.DragEffects = e.DataTransfer.Formats.Contains(DataFormat.File)
            ? DragDropEffects.Copy
            : DragDropEffects.None;

    private void OnDrop(object? sender, DragEventArgs e)
    {
        var path = AddProjectDialog.GetFirstLocalPath(e);
        if (path is not null)
        {
            (DataContext as AddLocalProjectDialogViewModel)?.SetDroppedFolder(path);
        }
    }

    private async void OnBrowseProjectFolder(object? sender, RoutedEventArgs e)
    {
        var path = await StoragePicker.PickFolderAsync(this, "选择已初始化的项目文件夹");
        if (path is not null && DataContext is AddLocalProjectDialogViewModel viewModel)
        {
            viewModel.ProjectPath = path;
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
