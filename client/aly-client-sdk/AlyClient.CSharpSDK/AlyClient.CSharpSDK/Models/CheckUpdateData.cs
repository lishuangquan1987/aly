using Newtonsoft.Json;

namespace AlyClient.CSharpSDK
{
    public class CheckUpdateData
    {
        [JsonProperty("has_update")]
        public bool HasUpdate { get; set; }

        [JsonProperty("need_download_update")]
        public bool NeedDownloadUpdate { get; set; }

        [JsonProperty("current_version")]
        public string CurrentVersion { get; set; }

        [JsonProperty("new_version")]
        public string NewVersion { get; set; }

        [JsonProperty("force_update")]
        public bool ForceUpdate { get; set; }
    }
}
