using Flurl.Http;
using PublishTool.Models;

namespace PublishTool.Services;

public class ProjectService
{
    public async Task<CommonResponse<List<ProjectDto>>> GetAllProjectsAsync(string serverUrl)
    {
        return await $"{serverUrl}/api/project/get_all_projects"
            .GetJsonAsync<CommonResponse<List<ProjectDto>>>();
    }

    public async Task<CommonResponse<ProjectDto>> CreateProjectAsync(string serverUrl, CreateProjectDto dto)
    {
        return await $"{serverUrl}/api/project/create_project"
            .PostJsonAsync(dto)
            .ReceiveJson<CommonResponse<ProjectDto>>();
    }

    public async Task<CommonResponse<ProjectDto>> UpdateProjectAsync(string serverUrl, UpdateProjectDto dto)
    {
        return await $"{serverUrl}/api/project/update_project"
            .PostJsonAsync(dto)
            .ReceiveJson<CommonResponse<ProjectDto>>();
    }

    public async Task<CommonResponse> DeleteProjectAsync(string serverUrl, int projectId)
    {
        return await $"{serverUrl}/api/project/delete_project/{projectId}"
            .PostAsync()
            .ReceiveJson<CommonResponse>();
    }

    public async Task<CommonResponse<List<ProjectChangeLog>>> GetChangeLogsAsync(string serverUrl, int projectId)
    {
        return await $"{serverUrl}/api/project/get_project_change_logs/{projectId}"
            .GetJsonAsync<CommonResponse<List<ProjectChangeLog>>>();
    }

    public async Task<CommonResponse<List<ServerOsInfoDto>>> GetOsInfoAsync(string serverUrl, int projectId)
    {
        return await $"{serverUrl}/api/project/get_project_os_info/{projectId}"
            .GetJsonAsync<CommonResponse<List<ServerOsInfoDto>>>();
    }
}
