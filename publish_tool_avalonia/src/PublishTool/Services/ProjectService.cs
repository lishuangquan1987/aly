using Flurl.Http;
using PublishTool.Models;
using Serilog;

namespace PublishTool.Services;

public class ProjectService
{
    public async Task<CommonResponse<List<ProjectDto>>> GetAllProjectsAsync(string serverUrl)
    {
        try
        {
            var response = await $"{serverUrl}/api/project/get_all_projects"
                .GetJsonAsync<CommonResponse<List<ProjectDto>>>();
            Log.Debug("获取项目列表成功, 服务器: {Url}", serverUrl);
            return response;
        }
        catch (FlurlHttpException ex)
        {
            Log.Error(ex, "获取项目列表失败, 服务器: {Url}", serverUrl);
            throw;
        }
        catch (Exception ex)
        {
            Log.Error(ex, "获取项目列表时发生未知错误, 服务器: {Url}", serverUrl);
            throw;
        }
    }

    public async Task<CommonResponse<ProjectDto>> CreateProjectAsync(string serverUrl, CreateProjectDto dto)
    {
        try
        {
            var response = await $"{serverUrl}/api/project/create_project"
                .PostJsonAsync(dto)
                .ReceiveJson<CommonResponse<ProjectDto>>();
            Log.Debug("创建项目成功, 服务器: {Url}", serverUrl);
            return response;
        }
        catch (FlurlHttpException ex)
        {
            Log.Error(ex, "创建项目失败, 服务器: {Url}", serverUrl);
            throw;
        }
        catch (Exception ex)
        {
            Log.Error(ex, "创建项目时发生未知错误, 服务器: {Url}", serverUrl);
            throw;
        }
    }

    public async Task<CommonResponse<ProjectDto>> UpdateProjectAsync(string serverUrl, UpdateProjectDto dto)
    {
        try
        {
            var response = await $"{serverUrl}/api/project/update_project"
                .PostJsonAsync(dto)
                .ReceiveJson<CommonResponse<ProjectDto>>();
            Log.Debug("更新项目成功, 服务器: {Url}", serverUrl);
            return response;
        }
        catch (FlurlHttpException ex)
        {
            Log.Error(ex, "更新项目失败, 服务器: {Url}", serverUrl);
            throw;
        }
        catch (Exception ex)
        {
            Log.Error(ex, "更新项目时发生未知错误, 服务器: {Url}", serverUrl);
            throw;
        }
    }

    public async Task<CommonResponse> DeleteProjectAsync(string serverUrl, int projectId)
    {
        try
        {
            var response = await $"{serverUrl}/api/project/delete_project/{projectId}"
                .PostAsync()
                .ReceiveJson<CommonResponse>();
            Log.Debug("删除项目成功, 项目ID: {ProjectId}", projectId);
            return response;
        }
        catch (FlurlHttpException ex)
        {
            Log.Error(ex, "删除项目失败, 项目ID: {ProjectId}", projectId);
            throw;
        }
        catch (Exception ex)
        {
            Log.Error(ex, "删除项目时发生未知错误, 项目ID: {ProjectId}", projectId);
            throw;
        }
    }

    public async Task<CommonResponse<List<ProjectChangeLogDto>>> GetChangeLogsAsync(string serverUrl, int projectId)
    {
        try
        {
            var response = await $"{serverUrl}/api/project/get_project_change_logs/{projectId}"
                .GetJsonAsync<CommonResponse<List<ProjectChangeLogDto>>>();
            Log.Debug("获取变更日志成功, 项目ID: {ProjectId}", projectId);
            return response;
        }
        catch (FlurlHttpException ex)
        {
            Log.Error(ex, "获取变更日志失败, 项目ID: {ProjectId}", projectId);
            throw;
        }
        catch (Exception ex)
        {
            Log.Error(ex, "获取变更日志时发生未知错误, 项目ID: {ProjectId}", projectId);
            throw;
        }
    }

    public async Task<CommonResponse<List<ServerOsInfoDto>>> GetProjectOsInfoAsync(string serverUrl, int projectId)
    {
        try
        {
            var response = await $"{serverUrl}/api/project/get_project_os_info/{projectId}"
                .GetJsonAsync<CommonResponse<List<ServerOsInfoDto>>>();
            Log.Debug("获取服务器信息成功, 项目ID: {ProjectId}", projectId);
            return response;
        }
        catch (FlurlHttpException ex)
        {
            Log.Error(ex, "获取服务器信息失败, 项目ID: {ProjectId}", projectId);
            throw;
        }
        catch (Exception ex)
        {
            Log.Error(ex, "获取服务器信息时发生未知错误, 项目ID: {ProjectId}", projectId);
            throw;
        }
    }
}
