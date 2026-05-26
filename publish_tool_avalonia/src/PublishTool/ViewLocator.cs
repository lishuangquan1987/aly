using System;
using Avalonia.Controls;
using Avalonia.Controls.Templates;

namespace PublishTool;

public class ViewLocator : IDataTemplate
{
    public Control? Build(object? param)
    {
        if (param is null) return null;

        var viewModelType = param.GetType();
        var viewTypeName = viewModelType.FullName!.Replace("ViewModels", "Views")
            .Replace("ViewModel", "View");
        var viewType = Type.GetType(viewTypeName);

        if (viewType != null)
        {
            return (Control)Activator.CreateInstance(viewType)!;
        }

        return new TextBlock { Text = $"View not found: {viewTypeName}" };
    }

    public bool Match(object? data) => data is not null;
}
