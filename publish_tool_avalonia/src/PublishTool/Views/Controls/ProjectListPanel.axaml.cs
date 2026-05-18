using Avalonia.Controls;
using Avalonia.Interactivity;
using PublishTool.Views;

namespace PublishTool.Views.Controls;

public partial class ProjectListPanel : UserControl
{
    public ProjectListPanel()
    {
        InitializeComponent();
    }

    private void ActionButton_Click(object? sender, RoutedEventArgs e)
    {
        if (sender is Button btn && btn.Tag is string action)
        {
            if (VisualRoot is MainWindow window)
            {
                switch (action)
                {
                    case "add":
                        _ = window.ShowAddClientProjectDialog();
                        break;
                    case "addserver":
                        _ = window.ShowAddServerProjectDialog();
                        break;
                }
            }
        }
    }
}
