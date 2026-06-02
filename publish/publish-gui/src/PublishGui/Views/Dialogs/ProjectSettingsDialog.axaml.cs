using Avalonia.Controls;
using Avalonia.Interactivity;
using Avalonia.Platform.Storage;
using PublishGui.Models.Local;
using System;
using System.Linq;

namespace PublishGui.Views.Dialogs;

public partial class ProjectSettingsDialog : Window
{
    public ProjectConfig? Result { get; private set; }

    public ProjectSettingsDialog()
    {
        InitializeComponent();
        CancelButton.Click += (_, _) => Close(null);
        SaveButton.Click += OnSave;
        BrowseButton.Click += OnBrowse;
        BrowseCliButton.Click += OnBrowseCli;
    }

    public void LoadConfig(ProjectConfig config)
    {
        ProjectNameBox.Text = config.ProjectName;
        ServerUrlBox.Text = config.ServerUrl;
        LocalPathBox.Text = config.ProjectPath;
        ProjectIdBox.Text = config.ProjectId.ToString();
        CliPathBox.Text = config.PublishCliPath;
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

    private async void OnBrowseCli(object? sender, RoutedEventArgs e)
    {
        var topLevel = GetTopLevel(this);
        if (topLevel == null) return;

        var files = await topLevel.StorageProvider.OpenFilePickerAsync(new FilePickerOpenOptions
        {
            Title = "选择 publish-cli.exe",
            AllowMultiple = false,
            FileTypeFilter = new[] { new FilePickerFileType("Executable") { Patterns = new[] { "*.exe" } } }
        });

        if (files.Count > 0)
        {
            CliPathBox.Text = files[0].Path.LocalPath;
        }
    }

    private void OnSave(object? sender, RoutedEventArgs e)
    {
        if (string.IsNullOrWhiteSpace(ProjectNameBox.Text) || 
            string.IsNullOrWhiteSpace(ServerUrlBox.Text) || 
            string.IsNullOrWhiteSpace(LocalPathBox.Text))
        {
            // TODO: Show error
            return;
        }

        int.TryParse(ProjectIdBox.Text, out var id);

        Result = new ProjectConfig
        {
            ProjectName = ProjectNameBox.Text,
            ServerUrl = ServerUrlBox.Text,
            ProjectPath = LocalPathBox.Text,
            ProjectId = id,
            PublishCliPath = CliPathBox.Text ?? string.Empty
        };

        Close(Result);
    }
}
