using Newtonsoft.Json;

namespace PublishTool.Models;

public class ServerOsInfoDto
{
    [JsonProperty("OS")]
    public string? Os { get; set; }

    [JsonProperty("platform")]
    public string? Platform { get; set; }

    [JsonProperty("goARCH")]
    public string? GoArch { get; set; }

    [JsonProperty("version")]
    public string? Version { get; set; }

    [JsonProperty("numCPU")]
    public int NumCpu { get; set; }

    [JsonProperty("cpuName")]
    public string? CpuName { get; set; }

    [JsonProperty("cpuMhz")]
    public double CpuMhz { get; set; }

    [JsonProperty("diskUsed")]
    public ulong DiskUsed { get; set; }

    [JsonProperty("diskFree")]
    public ulong DiskFree { get; set; }

    [JsonProperty("diskTotal")]
    public ulong DiskTotal { get; set; }

    [JsonProperty("diskUsedPercent")]
    public double DiskUsedPercent { get; set; }
}
