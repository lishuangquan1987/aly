using Newtonsoft.Json;

namespace PublishGui.Models.Cli;

/// <summary>
/// 服务器操作系统信息
/// </summary>
public class ServerOsInfo
{
    /// <summary>操作系统</summary>
    [JsonProperty("os")]
    public string OS { get; set; } = string.Empty;

    /// <summary>平台</summary>
    [JsonProperty("platform")]
    public string Platform { get; set; } = string.Empty;

    /// <summary>系统架构</summary>
    [JsonProperty("goARCH")]
    public string GoARCH { get; set; } = string.Empty;

    /// <summary>Go 版本</summary>
    [JsonProperty("version")]
    public string Version { get; set; } = string.Empty;

    /// <summary>CPU 核心数</summary>
    [JsonProperty("numCPU")]
    public int NumCPU { get; set; }

    /// <summary>CPU 名称</summary>
    [JsonProperty("cpuName")]
    public string CpuName { get; set; } = string.Empty;

    /// <summary>CPU 频率 (MHz)</summary>
    [JsonProperty("cpuMhz")]
    public double CpuMhz { get; set; }

    /// <summary>磁盘已用空间 (GB)</summary>
    [JsonProperty("diskUsed")]
    public double DiskUsed { get; set; }

    /// <summary>磁盘可用空间 (GB)</summary>
    [JsonProperty("diskFree")]
    public double DiskFree { get; set; }

    /// <summary>磁盘总空间 (GB)</summary>
    [JsonProperty("diskTotal")]
    public double DiskTotal { get; set; }

    /// <summary>磁盘使用率 (%)</summary>
    [JsonProperty("diskUsedPercent")]
    public double DiskUsedPercent { get; set; }
}
