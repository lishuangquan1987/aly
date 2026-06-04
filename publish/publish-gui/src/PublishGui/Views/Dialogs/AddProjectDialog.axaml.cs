using System;
using Avalonia.Controls;
using Avalonia.Interactivity;
using Avalonia.Platform.Storage;
using PublishGui.Models.Local;
using PublishGui.Services;

namespace PublishGui.Views.Dialogs;

public partial class AddProjectDialog : Window
{
    public AddProjectDialog()
    {
        InitializeComponent();
        ConfirmBtn.Click += OnConfirm;
        CancelBtn.Click += (_, _) => Close(null);
        BrowseBtn.Click += OnBrowse;
    }

    private async void OnBrowse(object? sender, RoutedEventArgs e)
    {
        var folders = await StorageProvider.OpenFolderPickerAsync(
            new FolderPickerOpenOptions { Title = "Select project folder" });
        if (folders.Count > 0)
            ProjectPathBox.Text = folders[0].Path.LocalPath;
    }

    private void OnConfirm(object? sender, RoutedEventArgs e)
    {
        if (string.IsNullOrWhiteSpace(ServerUrlBox.Text) ||
            string.IsNullOrWhiteSpace(ProjectNameBox.Text) ||
            string.IsNullOrWhiteSpace(ProjectPathBox.Text))
        {
            var msg = new Window
            {
                Title = "Validation",
                Width = 300, Height = 100,
                WindowStartupLocation = WindowStartupLocation.CenterOwner,
                Content = new TextBlock
                {
                    Text = "Please fill in all required fields.",
                    Margin = new Avalonia.Thickness(16),
                    VerticalAlignment = Avalonia.Layout.VerticalAlignment.Center,
                    HorizontalAlignment = Avalonia.Layout.HorizontalAlignment.Center
                }
            };
            msg.ShowDialog(this);
            return;
        }

        var cfg = new ProjectConfig
        {
            ServerUrl = ServerUrlBox.Text.Trim(),
            ProjectName = ProjectNameBox.Text.Trim(),
            ProjectPath = ProjectPathBox.Text.Trim(),
            ProjectId = int.TryParse(ProjectIdBox.Text, out var id) ? id : 0
        };

        var cfgService = new ConfigService();
        cfgService.AddProject(cfg);
        Close(cfg);
    }
}
