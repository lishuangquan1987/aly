using Avalonia.Controls;
using PublishTool.ViewModels;

namespace PublishTool.Views.Controls;

public partial class ProjectListPanel : UserControl
{
    public ProjectListPanel()
    {
        InitializeComponent();

        DataContextChanged += OnDataContextChanged;
    }

    private void OnDataContextChanged(object? sender, System.EventArgs e)
    {
        if (DataContext is MainWindowViewModel vm)
        {
            vm.FilteredProjects.CollectionChanged += (_, _) => UpdateEmptyState();
            UpdateEmptyState();
        }
    }

    private void UpdateEmptyState()
    {
        if (DataContext is MainWindowViewModel vm)
        {
            EmptyOverlay.IsVisible = vm.FilteredProjects.Count == 0;
        }
    }
}
