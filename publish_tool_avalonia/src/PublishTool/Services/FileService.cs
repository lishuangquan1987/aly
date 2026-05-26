using System;
using System.Collections.Generic;
using System.IO;
using System.Threading.Tasks;
using Flurl.Http;
using PublishTool.Models;
using Serilog;

namespace PublishTool.Services;

public class FileService
{
    public async Task<CommonResponse<List<FileInfoDto>>> GetAllFilesAsync(string serverUrl, int projectId)
    {
        try
        {
            var response = await $"{serverUrl}/api/file/get_all_files/{projectId}"
                .WithTimeout(60)
                .GetJsonAsync<CommonResponse<List<FileInfoDto>>>();
            Log.Debug("获取文件列表成功，项目ID: {ProjectId}, 文件数: {Count}",
                projectId, response.Data?.Count ?? 0);
            return response;
        }
        catch (FlurlHttpException ex)
        {
            Log.Error(ex, "获取文件列表失败: {Url}", $"{serverUrl}/api/file/get_all_files/{projectId}");
            throw;
        }
        catch (Exception ex)
        {
            Log.Error(ex, "获取文件列表时发生未知错误，项目ID: {ProjectId}", projectId);
            throw;
        }
    }

    public async Task<CommonResponse> UploadFileAsync(string serverUrl, string projectName,
        string relativePath, Stream fileStream)
    {
        try
        {
            Log.Information("开始上传文件: {Path} 到项目 {Project}", relativePath, projectName);
            var response = await $"{serverUrl}/api/file/upload_file"
                .WithTimeout(300)
                .PostMultipartAsync(mp => mp
                    .AddFile("file", fileStream, Path.GetFileName(relativePath))
                    .AddString("projectName", projectName)
                    .AddString("relativeFileName", relativePath))
                .ReceiveJson<CommonResponse>();
            Log.Information("文件上传成功: {Path}", relativePath);
            return response;
        }
        catch (FlurlHttpException ex)
        {
            Log.Error(ex, "文件上传失败: {Path}", relativePath);
            throw;
        }
        catch (Exception ex)
        {
            Log.Error(ex, "文件上传时发生未知错误: {Path}", relativePath);
            throw;
        }
    }

    public async Task<Stream> DownloadFileAsync(string serverUrl, string filePath)
    {
        try
        {
            Log.Debug("开始下载文件: {Path}", filePath);
            var stream = await $"{serverUrl}/api/file/download_file"
                .WithTimeout(300)
                .SetQueryParam("path", filePath)
                .GetStreamAsync();
            Log.Debug("文件下载成功: {Path}", filePath);
            return stream;
        }
        catch (FlurlHttpException ex)
        {
            Log.Error(ex, "文件下载失败: {Path}", filePath);
            throw;
        }
        catch (Exception ex)
        {
            Log.Error(ex, "文件下载时发生未知错误: {Path}", filePath);
            throw;
        }
    }
}
