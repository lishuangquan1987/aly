using Newtonsoft.Json;

namespace AlyClient.CSharpSDK
{
    internal class DownloadProgressData
    {
        [JsonProperty("index")]
        public int Index { get; set; }

        [JsonProperty("total")]
        public int Total { get; set; }

        [JsonProperty("file")]
        public string File { get; set; }

        [JsonProperty("status")]
        public string Status { get; set; }

        [JsonProperty("file_size")]
        public double FileSize { get; set; }
    }
}
