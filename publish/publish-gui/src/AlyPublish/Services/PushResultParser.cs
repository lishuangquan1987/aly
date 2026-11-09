using AlyPublish.Models.Cli;
using Newtonsoft.Json;
using Newtonsoft.Json.Linq;

namespace AlyPublish.Services;

/// <summary>
/// Classifies the mixed stream produced by <c>push --json</c>: per-file progress lines,
/// one final result line, and non-JSON noise.
/// </summary>
public static class PushResultParser
{
    public static PushLine Parse(string line)
    {
        JObject json;
        try
        {
            json = JObject.Parse(line);
        }
        catch (JsonException)
        {
            return new PushLine(PushLineKind.Ignored, null, null);
        }

        var data = json["data"];
        if (data is JObject dataObject && dataObject["status"] is not null)
        {
            var progress = dataObject.ToObject<UploadProgress>();
            if (progress is null)
            {
                return new PushLine(PushLineKind.Ignored, null, null);
            }

            var error = progress.Status == "FAIL" && !string.IsNullOrWhiteSpace(progress.Error)
                ? $"{progress.File}: {progress.Error}"
                : null;
            return new PushLine(PushLineKind.Progress, progress, error);
        }

        var isSuccess = json["isSuccess"]?.Value<bool>() ?? false;
        if (!isSuccess)
        {
            var errorMessage = json["errorMsg"]?.Value<string>();
            if (string.IsNullOrWhiteSpace(errorMessage) && data is JObject failureData)
            {
                errorMessage = failureData["error"]?.Value<string>();
            }

            return new PushLine(
                PushLineKind.Failure,
                null,
                string.IsNullOrWhiteSpace(errorMessage) ? "推送失败" : errorMessage);
        }

        return data is JObject
            ? new PushLine(PushLineKind.Success, null, null)
            : new PushLine(PushLineKind.Ignored, null, null);
    }
}
