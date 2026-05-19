using Newtonsoft.Json;

namespace PublishTool.Models;

public class ServerOsInfoDto
{
    [JsonProperty("os")]
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
    public double DiskUsed { get; set; }

    [JsonProperty("diskFree")]
    public double DiskFree { get; set; }

    [JsonProperty("diskTotal")]
    public double DiskTotal { get; set; }

    [JsonProperty("diskUsedPercent")]
    public double DiskUsedPercent { get; set; }
}
