using System;
using System.Threading.Tasks;
using CommunityToolkit.Mvvm.ComponentModel;
using Flurl.Http;
using PublishTool.Models;
using PublishTool.Services;

namespace PublishTool.ViewModels.Pages;

public partial class AddServerProjectDialogViewModel : ObservableObject
{
    private readonly ProjectService _projectService;

    [ObservableProperty]
    private string _serverUrl = string.Empty;

    [ObservableProperty]
    private string _name = string.Empty;

    [ObservableProperty]
    private string _title = string.Empty;

    [ObservableProperty]
    private bool _isForceUpdate;

    [ObservableProperty]
    private bool _isCreating;

    [ObservableProperty]
    private string _statusMessage = string.Empty;

    [ObservableProperty]
    private string _serverUrlError = string.Empty;

    [ObservableProperty]
    private string _nameError = string.Empty;

    [ObservableProperty]
    private string _titleError = string.Empty;

    partial void OnServerUrlChanged(string value) => ServerUrlError = string.Empty;
    partial void OnNameChanged(string value) => NameError = string.Empty;
    partial void OnTitleChanged(string value) => TitleError = string.Empty;

    public AddServerProjectDialogViewModel(ProjectService projectService)
    {
        _projectService = projectService;
    }

    public bool Validate()
    {
        var valid = true;
        if (string.IsNullOrWhiteSpace(ServerUrl))
        {
            ServerUrlError = "请输入服务器地址";
            valid = false;
        }
        else if (!ServerUrl.StartsWith("http://") && !ServerUrl.StartsWith("https://"))
        {
            ServerUrlError = "地址需以 http:// 或 https:// 开头";
            valid = false;
        }

        if (string.IsNullOrWhiteSpace(Name))
        {
            NameError = "请输入项目名称";
            valid = false;
        }

        if (string.IsNullOrWhiteSpace(Title))
        {
            TitleError = "请输入显示标题";
            valid = false;
        }

        return valid;
    }

    public async Task<bool> TryCreateAsync()
    {
        if (!Validate()) return false;

        IsCreating = true;
        StatusMessage = string.Empty;
        try
        {
            var dto = new CreateProjectDto
            {
                Name = Name,
                Title = Title,
                IsForceUpdate = IsForceUpdate
            };
            var response = await _projectService.CreateProjectAsync(ServerUrl, dto);
            if (response.IsSuccess)
            {
                StatusMessage = "创建成功";
                return true;
            }
            else
            {
                StatusMessage = $"创建失败: {response.ErrorMsg}";
                return false;
            }
        }
        catch (FlurlHttpException ex)
        {
            StatusMessage = $"网络错误: {ex.Message}";
            return false;
        }
        catch (Exception ex)
        {
            StatusMessage = $"创建失败: {ex.Message}";
            return false;
        }
        finally
        {
            IsCreating = false;
        }
    }
}
