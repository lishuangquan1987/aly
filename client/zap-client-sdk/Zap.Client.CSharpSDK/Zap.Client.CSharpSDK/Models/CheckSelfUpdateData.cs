using Newtonsoft.Json;

namespace Zap.Client.CSharpSDK
{
    public class CheckSelfUpdateData
    {
        [JsonProperty("need_update")]
        public bool NeedUpdate { get; set; }
    }
}
