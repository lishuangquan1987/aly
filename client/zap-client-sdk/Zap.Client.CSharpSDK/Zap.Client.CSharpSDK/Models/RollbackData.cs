using Newtonsoft.Json;

namespace Zap.Client.CSharpSDK
{
    public class RollbackData
    {
        [JsonProperty("version")]
        public string Version { get; set; }
    }
}
