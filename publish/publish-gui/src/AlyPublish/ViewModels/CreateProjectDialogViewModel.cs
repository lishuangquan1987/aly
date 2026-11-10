using System.Collections.ObjectModel;
using AlyPublish.Constants;
using AlyPublish.Localization;
using AlyPublish.Models.Cli;
using AlyPublish.Models.Local;
using AlyPublish.Services;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;

namespace AlyPublish.ViewModels;

/// <summary>Creates a server-side project. The identifier follows the title until the user edits it.</summary>
public sealed partial class CreateProjectDialogViewModel : ViewModelBase
{
    private readonly CliService _cli;
    private bool _syncingName;
    private bool _nameEdited;
    private IReadOnlyList<string> _existingProjectNames = [];

    public CreateProjectDialogViewModel(
        CliService cli,
        IDialogService dialogs,
        INotificationService notifications)
        : base(dialogs, notifications)
    {
        _cli = cli;
    }

    public ObservableCollection<string> IgnoreFolders { get; } = [];

    public ObservableCollection<string> IgnoreFiles { get; } = [];

    public IReadOnlyList<string> IgnoreFolderSuggestions { get; } = AppConstants.IgnoreFolderSuggestions;

    public IReadOnlyList<string> IgnoreFileSuggestions { get; } = AppConstants.IgnoreFileSuggestions;

    [ObservableProperty]
    private string _serverUrl = string.Empty;

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(CreateCommand))]
    private string _projectTitle = string.Empty;

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(CreateCommand))]
    private string _projectName = string.Empty;

    [ObservableProperty]
    private bool _forceUpdate;

    [ObservableProperty]
    [NotifyCanExecuteChangedFor(nameof(CreateCommand))]
    [NotifyPropertyChangedFor(nameof(HasValidationMessage))]
    private string _validationMessage = string.Empty;

    public bool HasValidationMessage => !string.IsNullOrEmpty(ValidationMessage);

    public void Apply(CreateServerProjectRequest request)
    {
        ServerUrl = request.ServerUrl;
        _existingProjectNames = request.ExistingProjectNames;

        foreach (var folder in request.SuggestedIgnoreFolders)
        {
            AddIfMissing(IgnoreFolders, folder);
        }

        foreach (var file in request.SuggestedIgnoreFiles)
        {
            AddIfMissing(IgnoreFiles, file);
        }

        UpdateValidation();
    }

    partial void OnProjectTitleChanged(string value)
    {
        if (!_nameEdited)
        {
            _syncingName = true;
            ProjectName = value.Trim();
            _syncingName = false;
        }

        UpdateValidation();
    }

    partial void OnProjectNameChanged(string value)
    {
        if (!_syncingName)
        {
            _nameEdited = true;
        }

        UpdateValidation();
    }

    protected override void OnBusyChanged() => CreateCommand.NotifyCanExecuteChanged();

    [RelayCommand(CanExecute = nameof(CanCreate))]
    private async Task CreateAsync()
    {
        var name = ProjectName.Trim();
        var title = ProjectTitle.Trim();
        ProjectInfo? created = null;

        var saved = await RunAsync(LanguageManager.T(LanguageManager.Instance.Op_CreateServerProject), async token =>
        {
            created = await _cli.CreateServerProjectAsync(
                ServerUrl,
                name,
                title,
                ForceUpdate,
                Clean(IgnoreFolders),
                Clean(IgnoreFiles),
                token);
        });

        if (saved && created is not null)
        {
            Close(created);
        }
    }

    [RelayCommand]
    private void AddSuggestedIgnoreFolder(string? value) => AddIfMissing(IgnoreFolders, value);

    [RelayCommand]
    private void AddSuggestedIgnoreFile(string? value) => AddIfMissing(IgnoreFiles, value);

    [RelayCommand]
    private void Cancel() => Close(null);

    private bool CanCreate()
        => !IsBusy
           && !string.IsNullOrWhiteSpace(ProjectTitle)
           && !string.IsNullOrWhiteSpace(ProjectName)
           && string.IsNullOrEmpty(ValidationMessage);

    private void UpdateValidation()
    {
        var name = ProjectName.Trim();
        ValidationMessage = name.Length > 0 && _existingProjectNames.Any(existing =>
            string.Equals(existing, name, StringComparison.OrdinalIgnoreCase))
            ? LanguageManager.T(LanguageManager.Instance.Dlg_DuplicateProject)
            : string.Empty;

        CreateCommand.NotifyCanExecuteChanged();
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

    private static List<string> Clean(IEnumerable<string> values)
        => values
            .Select(value => value.Trim())
            .Where(value => value.Length > 0)
            .Distinct(StringComparer.Ordinal)
            .ToList();
}
