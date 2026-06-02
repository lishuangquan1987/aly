using Avalonia.Controls;
using Avalonia.Interactivity;
using Avalonia.Platform.Storage;
using System;

namespace PublishGui.Views.Dialogs;

/// <summary>
/// 首次运行向导
/// 引导用户添加或创建第一个项目
/// </summary>
public partial class FirstRunWizard : Window
{
    private enum WizardMode
    {
        ChooseMode,
        AddExisting,
        CreateNew
    }

    private WizardMode _currentMode = WizardMode.ChooseMode;

    /// <summary>向导结果</summary>
    public WizardResult? Result { get; private set; }

    public FirstRunWizard()
    {
        InitializeComponent();
        
        SkipButton.Click += (_, _) => Close(null);
        BackButton.Click += OnBack;
        NextButton.Click += OnNext;
        BrowsePathButton.Click += OnBrowsePath;
        NewBrowsePathButton.Click += OnBrowseNewPath;
        
        AddExistingOption.PointerPressed += (_, _) => SelectMode(WizardMode.AddExisting);
        CreateNewOption.PointerPressed += (_, _) => SelectMode(WizardMode.CreateNew);
    }

    private void SelectMode(WizardMode mode)
    {
        _currentMode = mode;
        
        if (mode == WizardMode.AddExisting)
        {
            StepsTabControl.SelectedIndex = 1;
            BackButton.IsVisible = true;
        }
        else if (mode == WizardMode.CreateNew)
        {
            StepsTabControl.SelectedIndex = 2;
            BackButton.IsVisible = true;
        }
    }

    private void OnBack(object? sender, RoutedEventArgs e)
    {
        _currentMode = WizardMode.ChooseMode;
        StepsTabControl.SelectedIndex = 0;
        BackButton.IsVisible = false;
    }

    private void OnNext(object? sender, RoutedEventArgs e)
    {
        if (_currentMode == WizardMode.ChooseMode)
        {
            // 用户没有选择模式
            return;
        }

        if (_currentMode == WizardMode.AddExisting)
        {
            // 验证输入
            if (string.IsNullOrWhiteSpace(ProjectNameBox.Text) ||
                string.IsNullOrWhiteSpace(ServerUrlBox.Text) ||
                string.IsNullOrWhiteSpace(LocalPathBox.Text))
            {
                // TODO: 显示错误提示
                return;
            }

            int.TryParse(ProjectIdBox.Text, out var projectId);

            Result = new WizardResult
            {
                Mode = WizardResultMode.AddExisting,
                ProjectName = ProjectNameBox.Text,
                ServerUrl = ServerUrlBox.Text,
                LocalPath = LocalPathBox.Text,
                ProjectId = projectId
            };
        }
        else if (_currentMode == WizardMode.CreateNew)
        {
            // 验证输入
            if (string.IsNullOrWhiteSpace(NewServerUrlBox.Text) ||
                string.IsNullOrWhiteSpace(NewProjectNameBox.Text) ||
                string.IsNullOrWhiteSpace(NewProjectTitleBox.Text) ||
                string.IsNullOrWhiteSpace(NewLocalPathBox.Text))
            {
                // TODO: 显示错误提示
                return;
            }

            Result = new WizardResult
            {
                Mode = WizardResultMode.CreateNew,
                ServerUrl = NewServerUrlBox.Text,
                ProjectName = NewProjectNameBox.Text,
                ProjectTitle = NewProjectTitleBox.Text,
                LocalPath = NewLocalPathBox.Text,
                ForceUpdate = ForceUpdateCheckBox.IsChecked ?? false
            };
        }

        Close(Result);
    }

    private async void OnBrowsePath(object? sender, RoutedEventArgs e)
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

    private async void OnBrowseNewPath(object? sender, RoutedEventArgs e)
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
            NewLocalPathBox.Text = folders[0].Path.LocalPath;
        }
    }
}

/// <summary>
/// 向导结果模式
/// </summary>
public enum WizardResultMode
{
    AddExisting,
    CreateNew
}

/// <summary>
/// 向导结果
/// </summary>
public class WizardResult
{
    public WizardResultMode Mode { get; set; }
    public string ServerUrl { get; set; } = string.Empty;
    public string ProjectName { get; set; } = string.Empty;
    public string ProjectTitle { get; set; } = string.Empty;
    public string LocalPath { get; set; } = string.Empty;
    public int ProjectId { get; set; }
    public bool ForceUpdate { get; set; }
}
