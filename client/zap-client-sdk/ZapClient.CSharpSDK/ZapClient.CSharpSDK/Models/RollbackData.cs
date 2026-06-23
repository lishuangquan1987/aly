using Newtonsoft.Json;

namespace ZapClient.CSharpSDK
{
    public class RollbackData
    {
        [JsonProperty("version")]
        public string Version { get; set; }
    }
}
