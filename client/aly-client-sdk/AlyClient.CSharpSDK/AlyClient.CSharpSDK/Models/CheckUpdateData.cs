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

        /// <summary>
        /// 新版本的发布内容（服务端变更日志，逐条一项）。服务端未配置时为 null。
        /// aly-client 仅在上报新版本（has_update=true 且需要下载）时下发。
        /// </summary>
        [JsonProperty("release_notes")]
        public string[] ReleaseNotes { get; set; }
    }
}
