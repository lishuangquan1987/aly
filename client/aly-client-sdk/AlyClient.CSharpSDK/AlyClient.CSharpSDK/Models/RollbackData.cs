using Newtonsoft.Json;

namespace AlyClient.CSharpSDK
{
    public class RollbackData
    {
        [JsonProperty("version")]
        public string Version { get; set; }
    }
}
