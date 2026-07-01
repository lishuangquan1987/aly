using Newtonsoft.Json;

namespace AlyClient.CSharpSDK
{
    public class CheckSelfUpdateData
    {
        [JsonProperty("need_update")]
        public bool NeedUpdate { get; set; }
    }
}
