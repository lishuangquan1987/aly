using Newtonsoft.Json;

namespace Zap.Client.CSharpSDK
{
    public class ListRollbackData
    {
        [JsonProperty("current_version")]
        public string CurrentVersion { get; set; }

        [JsonProperty("versions")]
        public string[] Versions { get; set; }
    }
}
