using Newtonsoft.Json;
using System;

namespace AlyClient.CSharpSDK.Models
{
    /// <summary>aly-client.exe stdout JSON wrapper (non-generic).</summary>
    public class AlyResponse
    {
        [JsonProperty("isSuccess")]
        public bool IsSuccess { get; set; }

        [JsonProperty("errorMsg")]
        public string ErrorMsg { get; set; }

        public static AlyResponse OK()
        {
            return new AlyResponse { IsSuccess = true };
        }

        public static AlyResponse NG(string errMsg)
        {
            return new AlyResponse { IsSuccess = false, ErrorMsg = errMsg };
        }

        public static AlyResponse NG(Exception e)
        {
            return new AlyResponse { IsSuccess = false, ErrorMsg = e.Message };
        }
    }

    /// <summary>aly-client.exe stdout JSON wrapper with typed data payload.</summary>
    public class AlyResponse<T> : AlyResponse
    {
        [JsonProperty("data")]
        public T Data { get; set; }

        public static AlyResponse<T> OK(T data)
        {
            return new AlyResponse<T> { IsSuccess = true, Data = data };
        }

        /// <summary>Create a failed response with an error message.</summary>
        public static new AlyResponse<T> NG(string errMsg)
        {
            return new AlyResponse<T> { IsSuccess = false, ErrorMsg = errMsg };
        }

        /// <summary>Create a failed response from an exception.</summary>
        public static new AlyResponse<T> NG(Exception e)
        {
            return new AlyResponse<T> { IsSuccess = false, ErrorMsg = e.Message };
        }
    }
}
