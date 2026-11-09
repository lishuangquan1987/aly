using Newtonsoft.Json;

namespace AlyClient.CSharpSDK.Models
{
    /// <summary>
    /// self_update 命令的 data：
    /// updated=false 时 reason 说明跳过原因（source_missing / already_up_to_date）；
    /// updated=true 时 old_path 携带被替换下来的旧版备份路径（.old）。
    /// </summary>
    public class SelfUpdateData
    {
        [JsonProperty("updated")]
        public bool Updated { get; set; }

        [JsonProperty("reason")]
        public string Reason { get; set; }

        [JsonProperty("old_path")]
        public string OldPath { get; set; }
    }
}