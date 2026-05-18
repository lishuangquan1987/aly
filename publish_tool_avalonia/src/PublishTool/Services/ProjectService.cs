using Flurl.Http;
using Newtonsoft.Json;
using PublishTool.Models;

namespace PublishTool.Services;

public class ProjectService
{
    public async Task<List<ProjectDto>> GetAllProjectsAsync(string serverUrl)
    {
        var response = await $"{serverUrl}/api/project/get_all_projects"
            .GetJsonAsync<CommonResponse<List<ProjectDto>>>();
        return response.Data ?? new List<ProjectDto>();
    }

    public async Task<ProjectDto?> CreateProjectAsync(string serverUrl, CreateProjectDto dto)
    {
        var response = await $"{serverUrl}/api/project/create_project"
            .PostJsonAsync(dto)
            .ReceiveJson<CommonResponse<ProjectDto>>();
        return response.Data;
    }

    public async Task<ProjectDto?> UpdateProjectAsync(string serverUrl, UpdateProjectDto dto)
    {
        var response = await $"{serverUrl}/api/project/update_project"
            .PostJsonAsync(dto)
            .ReceiveJson<CommonResponse<ProjectDto>>();
        return response.Data;
    }

    public async Task<bool> DeleteProjectAsync(string serverUrl, int projectId)
    {
        var response = await $"{serverUrl}/api/project/delete_project/{projectId}"
            .PostAsync()
            .ReceiveJson<CommonResponse>();
        return response.IsSuccess;
    }

    public async Task<List<ProjectChangeLog>> GetChangeLogsAsync(string serverUrl, int projectId)
    {
        var response = await $"{serverUrl}/api/project/get_project_change_logs/{projectId}"
            .GetJsonAsync<CommonResponse<List<ProjectChangeLog>>>();
        return response.Data ?? new List<ProjectChangeLog>();
    }

    public async Task<ServerOsInfoDto?> GetOsInfoAsync(string serverUrl, int projectId)
    {
        try
        {
            var response = await $"{serverUrl}/api/project/get_project_os_info/{projectId}"
                .GetJsonAsync<CommonResponse<ServerOsInfoDto>>();
            return response.Data;
        }
        catch
        {
            return null;
        }
    }
}
