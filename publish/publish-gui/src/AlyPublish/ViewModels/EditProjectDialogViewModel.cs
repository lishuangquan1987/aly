using System.Collections.ObjectModel;
using System.IO;
using AlyPublish.Helpers;
using AlyPublish.Localization;
using AlyPublish.Models.Local;
using AlyPublish.Services;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;
using Serilog;

namespace AlyPublish.ViewModels;

/// <summary>
/// Edits GUI fields and .updator/shared.json in one transaction. Local CLI writes must succeed
/// before the server sync is attempted; a sync failure is only a warning.
/// </summary>
public sealed partial class EditProjectDialogViewModel : ViewModelBase
{
    private readonly ConfigService _config;
    private readonly ProjectConfigService _projectConfig;
    private string _projectPath = string.Empty;

    public EditProjectDialogViewModel(
        ConfigService config,
        ProjectConfigService projectConfig,
        IDialogService dialogs,
        INotificationService notifications)
        : base(dialogs, notifications)
    {
        _config = config;
        _projectConfig = projectConfig;
    }

    public ObservableCollection<string> IgnoreFolders { get; } = [];

    public ObservableCollection<string> IgnoreFiles { get; } = [];

    public ObservableCollection<string> UnCopyFolders { get; } = [];

    public ObservableCollection<string> UnCopyFiles { get; } = [];

    public ObservableCollection<string> KnownServerUrls { get; } = [];

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(SaveCommand))]
    private string _displayName = string.Empty;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(MainExeHint))]
    private string _mainExePath = string.Empty;

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(SaveCommand))]
    private string _serverUrl = string.Empty;

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(SaveCommand))]
    private string _projectName = string.Empty;

    [ObservableProperty]
    private bool _isLoaded;

    [ObservableProperty]
    [NotifyPropertyChangedFor(nameof(HasLoadError))]
    private string _loadError = string.Empty;

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(SaveCommand))]
    [NotifyPropertyChangedFor(nameof(HasValidationMessage))]
    private string _validationMessage = string.Empty;

    public bool HasLoadError => !string.IsNullOrEmpty(LoadError);

    public bool HasValidationMessage => !string.IsNullOrEmpty(ValidationMessage);

    public string ProjectPath => _projectPath;

    public string MainExeHint
    {
        get
        {
            if (string.IsNullOrWhiteSpace(MainExePath))
            {
                return LanguageManager.T(LanguageManager.Instance.Dlg_MainExeOptional);
            }

            var stored = ProjectInputHelper.ToStoredMainExePath(_projectPath, MainExePath.Trim());
            return Path.IsPathRooted(stored)
            ? LanguageManager.T(LanguageManager.Instance.Dlg_MainExeOutside)
            : LanguageManager.F(LanguageManager.Instance.Dlg_MainExeWillSave, stored);
        }
    }

    public void Load(ProjectConfig project)
    {
        _projectPath = project.ProjectPath;
        DisplayName = project.DisplayName;
        MainExePath = project.MainExePath;

        var existingProjects = _config.LoadProjects();
        foreach (var url in ProjectConfigService.GetKnownServerUrls(existingProjects.Select(item => item.ProjectPath)))
        {
            AddIfMissing(KnownServerUrls, url);
        }

        try
        {
            var shared = ProjectConfigService.Load(_projectPath);
            ServerUrl = shared.ServerUrl;
            ProjectName = shared.ProjectName;
            Fill(IgnoreFolders, shared.IgnoreFolders);
            Fill(IgnoreFiles, shared.IgnoreFiles);
            Fill(UnCopyFolders, shared.UnCopyFolders);
            Fill(UnCopyFiles, shared.UnCopyFiles);
            AddIfMissing(KnownServerUrls, ServerUrl);
            IsLoaded = true;
        }
        catch (ProjectConfigException ex)
        {
            LoadError = ex.Message;
            IsLoaded = false;
            Log.Warning(ex, "加载项目配置失败: {Path}", _projectPath);
        }

        UpdateValidation();
    }

    partial void OnDisplayNameChanged(string value) => UpdateValidation();

    partial void OnServerUrlChanged(string value) => UpdateValidation();

    partial void OnProjectNameChanged(string value) => UpdateValidation();

    protected override void OnBusyChanged() => SaveCommand.NotifyCanExecuteChanged();

    [RelayCommand(CanExecute = nameof(CanSave))]
    private async Task SaveAsync()
    {
        var draft = new ProjectConfigDraft(
            ServerUrl.Trim(),
            ProjectName.Trim(),
            Clean(IgnoreFolders),
            Clean(IgnoreFiles),
            Clean(UnCopyFolders),
            Clean(UnCopyFiles));

        string? syncWarning = null;
        var saved = await RunAsync(LanguageManager.T(LanguageManager.Instance.Op_SaveSettings), async token =>
        {
            var changed = await _projectConfig.SaveAsync(_projectPath, draft, token);
            if (changed)
            {
                syncWarning = await _projectConfig.SyncToServerAsync(_projectPath, token);
            }
        });

        if (!saved)
        {
            return;
        }

        if (!string.IsNullOrWhiteSpace(syncWarning))
        {
            Notify.Warning(syncWarning);
        }

        Close(new ProjectConfig
        {
            DisplayName = DisplayName.Trim(),
            ProjectPath = _projectPath,
            MainExePath = ProjectInputHelper.ToStoredMainExePath(_projectPath, MainExePath.Trim())
        });
    }

    [RelayCommand]
    private void Cancel() => Close(null);

    [RelayCommand]
    private void UseKnownServerUrl(string? value)
    {
        if (!string.IsNullOrWhiteSpace(value))
        {
            ServerUrl = value.Trim();
        }
    }

    [RelayCommand]
    private void AddSuggestedIgnoreFolder(string? value) => AddIfMissing(IgnoreFolders, value);

    [RelayCommand]
    private void AddSuggestedIgnoreFile(string? value) => AddIfMissing(IgnoreFiles, value);

    public void AddIgnoreFolder(string fullPath) => AddProjectEntry(IgnoreFolders, fullPath, LanguageManager.T(LanguageManager.Instance.Kind_Folder));

    public void AddIgnoreFile(string fullPath) => AddProjectEntry(IgnoreFiles, fullPath, LanguageManager.T(LanguageManager.Instance.Kind_File));

    public void AddUnCopyFolder(string fullPath) => AddProjectEntry(UnCopyFolders, fullPath, LanguageManager.T(LanguageManager.Instance.Kind_Folder));

    public void AddUnCopyFile(string fullPath) => AddProjectEntry(UnCopyFiles, fullPath, LanguageManager.T(LanguageManager.Instance.Kind_File));

    private bool CanSave()
        => !IsBusy
           && IsLoaded
           && !string.IsNullOrWhiteSpace(DisplayName)
           && !string.IsNullOrWhiteSpace(ServerUrl)
           && !string.IsNullOrWhiteSpace(ProjectName)
           && string.IsNullOrEmpty(ValidationMessage);

    private void UpdateValidation()
    {
        ValidationMessage = string.IsNullOrWhiteSpace(ServerUrl)
            ? LanguageManager.T(LanguageManager.Instance.Dlg_ServerUrlRequired)
            : string.IsNullOrWhiteSpace(ProjectName)
                ? LanguageManager.T(LanguageManager.Instance.Dlg_ServerProjectRequired)
                : string.Empty;

        SaveCommand.NotifyCanExecuteChanged();
    }

    private static void Fill(ObservableCollection<string> target, IEnumerable<string> values)
    {
        target.Clear();
        foreach (var value in values)
        {
            if (!string.IsNullOrWhiteSpace(value))
            {
                target.Add(value);
            }
        }
    }

    private static void AddIfMissing(ObservableCollection<string> target, string? value)
    {
        if (string.IsNullOrWhiteSpace(value))
        {
            return;
        }

        var trimmed = value.Trim();
        if (!target.Contains(trimmed))
        {
            target.Add(trimmed);
        }
    }

    private void AddProjectEntry(ObservableCollection<string> target, string fullPath, string kind)
    {
        if (!ProjectInputHelper.TryGetProjectRelativePath(_projectPath, fullPath, out var relative))
        {
            Notify.Warning(LanguageManager.F(LanguageManager.Instance.Dlg_PickInProject, kind));
            return;
        }

        AddIfMissing(target, relative);
    }

    private static List<string> Clean(IEnumerable<string> values)
        => values
            .Select(value => value.Trim())
            .Where(value => value.Length > 0)
            .Distinct(StringComparer.Ordinal)
            .ToList();
}
