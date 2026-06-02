using Avalonia.Controls;
using Avalonia.Interactivity;
using System;

namespace PublishGui.Views.Dialogs;

public partial class AddProjectDialog : Window
{
    public string ProjectName { get; private set; } = string.Empty;
    public string ServerUrl { get; private set; } = string.Empty;
    public string LocalPath { get; private set; } = string.Empty;
    public int ProjectId { get; private set; }

    public AddProjectDialog()
    {
        InitializeComponent();
        CancelButton.Click += (_, _) => Close(null);
        ConfirmButton.Click += OnConfirm;
        BrowseButton.Click += OnBrowse;
    }

    private async void OnBrowse(object? sender, RoutedEventArgs e)
    {
        var dialog = new OpenFolderDialog();
        var result = await dialog.ShowAsync(this);
        if (!string.IsNullOrEmpty(result))
        {
            LocalPathBox.Text = result;
        }
    }

    private void OnConfirm(object? sender, RoutedEventArgs e)
    {
        ProjectName = ProjectNameBox.Text ?? string.Empty;
        ServerUrl = ServerUrlBox.Text ?? string.Empty;
        LocalPath = LocalPathBox.Text ?? string.Empty;
        int.TryParse(ProjectIdBox.Text, out var id);
        ProjectId = id;

        if (string.IsNullOrWhiteSpace(ProjectName) || string.IsNullOrWhiteSpace(ServerUrl) || string.IsNullOrWhiteSpace(LocalPath))
        {
            // TODO: Show error message
            return;
        }

        Close((ProjectName, ServerUrl, LocalPath, ProjectId));
    }
}
