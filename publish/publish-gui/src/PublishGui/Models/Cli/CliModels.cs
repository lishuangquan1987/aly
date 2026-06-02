using Newtonsoft.Json;

namespace PublishGui.Models.Cli;

public class CliOutput<T>
{
    [JsonProperty("isSuccess")]
    public bool IsSuccess { get; set; }

    [JsonProperty("errorMsg")]
    public string ErrorMsg { get; set; } = string.Empty;

    [JsonProperty("data")]
    public T? Data { get; set; }
}

public class FileStatusItem
{
    [JsonProperty("relativePath")]
    public string RelativePath { get; set; } = string.Empty;

    [JsonProperty("status")]
    public string Status { get; set; } = string.Empty;

    [JsonProperty("localMd5")]
    public string LocalMd5 { get; set; } = string.Empty;

    [JsonProperty("localSize")]
    public long LocalSize { get; set; }

    [JsonProperty("serverMd5")]
    public string ServerMd5 { get; set; } = string.Empty;

    [JsonProperty("serverSize")]
    public long ServerSize { get; set; }
}

public class StatusData
{
    [JsonProperty("staged")]
    public List<FileStatusItem> Staged { get; set; } = new();

    [JsonProperty("unstaged")]
    public List<FileStatusItem> Unstaged { get; set; } = new();

    [JsonProperty("unchanged")]
    public List<FileStatusItem> Unchanged { get; set; } = new();
}

public class ProjectInfo
{
    [JsonProperty("id")]
    public int Id { get; set; }

    [JsonProperty("name")]
    public string Name { get; set; } = string.Empty;

    [JsonProperty("title")]
    public string Title { get; set; } = string.Empty;

    [JsonProperty("version")]
    public string Version { get; set; } = string.Empty;

    [JsonProperty("force_update")]
    public bool ForceUpdate { get; set; }

    [JsonProperty("ignore_folders")]
    public List<string> IgnoreFolders { get; set; } = new();

    [JsonProperty("ignore_files")]
    public List<string> IgnoreFiles { get; set; } = new();

    [JsonProperty("created_at")]
    public string CreatedAt { get; set; } = string.Empty;

    [JsonProperty("is_deleted")]
    public bool IsDeleted { get; set; }
}

public class ChangeLog
{
    [JsonProperty("id")]
    public int Id { get; set; }

    [JsonProperty("version")]
    public string Version { get; set; } = string.Empty;

    [JsonProperty("logs")]
    public List<string> Logs { get; set; } = new();

    [JsonProperty("time")]
    public string Time { get; set; } = string.Empty;

    [JsonProperty("created_at")]
    public string CreatedAt { get; set; } = string.Empty;
}

public class ServerOsInfo
{
    [JsonProperty("os")]
    public string OS { get; set; } = string.Empty;

    [JsonProperty("platform")]
    public string Platform { get; set; } = string.Empty;

    [JsonProperty("goARCH")]
    public string GoARCH { get; set; } = string.Empty;

    [JsonProperty("version")]
    public string Version { get; set; } = string.Empty;

    [JsonProperty("numCPU")]
    public int NumCPU { get; set; }

    [JsonProperty("cpuName")]
    public string CpuName { get; set; } = string.Empty;

    [JsonProperty("cpuMhz")]
    public double CpuMhz { get; set; }

    [JsonProperty("diskUsed")]
    public double DiskUsed { get; set; }

    [JsonProperty("diskFree")]
    public double DiskFree { get; set; }

    [JsonProperty("diskTotal")]
    public double DiskTotal { get; set; }

    [JsonProperty("diskUsedPercent")]
    public double DiskUsedPercent { get; set; }
}
