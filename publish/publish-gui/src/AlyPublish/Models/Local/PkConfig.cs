using Newtonsoft.Json;

namespace AlyPublish.Models.Local;

/// <summary>
/// The contents of <c>payload.json</c>, written by the packager into the AlyPublish package root.
/// It names the packaged runtime the GUI itself runs as, and the publish-cli / updater copies it must use,
/// so the runtime never has to guest the platform.
/// </summary>
public sealed class PkConfig
{
    [JsonProperty("host_rid")]
    public string HostRid { get; set; } = string.Empty;

    [JsonProperty("cli_rid")]
    public string PubRid { get; set; } = string.Empty;

    [JsonProperty("client_rid")]
    public string UpRid { get; set; } = string.Empty;

    [JsonProperty("payload_rids")]
    public List<string> RidList { get; set; } = [];
}
