using Avalonia.Controls;
using Avalonia.Interactivity;
using Avalonia.Platform.Storage;
using System;

namespace PublishGui.Views.Dialogs;

public partial class CreateProjectDialog : Window
{
    public string ServerUrl { get; private set; } = string.Empty;
    public string ProjectName { get; private set; } = string.Empty;
    public string ProjectTitle { get; private set; } = string.Empty;
    public bool ForceUpdate { get; private set; }
    public string LocalPath { get; private set; } = string.Empty;

    public CreateProjectDialog()
    {
        InitializeComponent();
        CancelButton.Click += (_, _) => Close(null);
        CreateButton.Click += OnCreate;
        BrowseButton.Click += OnBrowse;
    }

    private async void OnBrowse(object? sender, RoutedEventArgs e)
    {
        var topLevel = GetTopLevel(this);
        if (topLevel == null) return;

        var folders = await topLevel.StorageProvider.OpenFolderPickerAsync(new FolderPickerOpenOptions
        {
            Title = "选择项目路径",
            AllowMultiple = false
        });

        if (folders.Count > 0)
        {
            LocalPathBox.Text = folders[0].Path.LocalPath;
        }
    }

    private void OnCreate(object? sender, RoutedEventArgs e)
    {
        ServerUrl = ServerUrlBox.Text ?? string.Empty;
        ProjectName = ProjectNameBox.Text ?? string.Empty;
        ProjectTitle = ProjectTitleBox.Text ?? string.Empty;
        ForceUpdate = ForceUpdateCheckBox.IsChecked ?? false;
        LocalPath = LocalPathBox.Text ?? string.Empty;

        if (string.IsNullOrWhiteSpace(ServerUrl) || 
            string.IsNullOrWhiteSpace(ProjectName) || 
            string.IsNullOrWhiteSpace(ProjectTitle) ||
            string.IsNullOrWhiteSpace(LocalPath))
        {
            // TODO: Show error
            return;
        }

        Close((ServerUrl, ProjectName, ProjectTitle, ForceUpdate, LocalPath));
    }
}
