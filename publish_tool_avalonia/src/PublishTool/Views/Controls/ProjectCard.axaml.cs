using Avalonia;
using Avalonia.Controls;
using Avalonia.Controls.Primitives;
using Avalonia.Input;
using Avalonia.Interactivity;
using PublishTool.Models.Local;
using PublishTool.ViewModels;
using PublishTool.Views;

namespace PublishTool.Views.Controls;

public partial class ProjectCard : UserControl
{
    public ProjectCard()
    {
        InitializeComponent();
    }

    private void CardPointerEnter(object? sender, PointerEventArgs e)
    {
        if (this.FindControl<StackPanel>("ActionButtons") is StackPanel panel)
        {
            foreach (var child in panel.Children)
            {
                if (child is Button btn)
                {
                    btn.Opacity = 1;
                }
            }
        }
    }

    private void CardPointerLeave(object? sender, PointerEventArgs e)
    {
        if (this.FindControl<StackPanel>("ActionButtons") is StackPanel panel)
        {
            foreach (var child in panel.Children)
            {
                if (child is Button btn)
                {
                    btn.Opacity = 0;
                }
            }
        }
    }

    private void ActionButton_Click(object? sender, RoutedEventArgs e)
    {
        if (sender is Button btn && btn.Tag is string action)
        {
            var project = DataContext as ProjectConfig;
            if (project != null && VisualRoot is MainWindow window && window.DataContext is MainWindowViewModel vm)
            {
                switch (action)
                {
                    case "moveup":
                        var upIndex = vm.FilteredProjects.IndexOf(project);
                        if (upIndex > 0)
                            vm.MoveUpProjectCommand.Execute(upIndex);
                        break;
                    case "movedown":
                        var downIndex = vm.FilteredProjects.IndexOf(project);
                        if (downIndex >= 0 && downIndex < vm.FilteredProjects.Count - 1)
                            vm.MoveDownProjectCommand.Execute(downIndex);
                        break;
                    case "delete":
                        vm.RemoveProjectCommand.Execute(project);
                        break;
                }
            }
        }
    }
}