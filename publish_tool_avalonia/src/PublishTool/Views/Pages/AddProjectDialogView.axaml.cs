using System;
using System.Linq;
using Avalonia.Controls;
using Avalonia.Interactivity;
using Avalonia.Platform.Storage;
using AddProjectDialogViewModel = PublishTool.ViewModels.Pages.AddProjectDialogViewModel;

namespace PublishTool.Views.Pages;

public partial class AddProjectDialogView : UserControl
{
    public AddProjectDialogView()
    {
        InitializeComponent();
    }
}
