using Newtonsoft.Json;

namespace ZapClient.CSharpSDK
{
    public class CheckSelfUpdateData
    {
        [JsonProperty("need_update")]
        public bool NeedUpdate { get; set; }
    }
}
