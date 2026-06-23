using Newtonsoft.Json;
using System;

namespace ZapClient.CSharpSDK.Models
{
    /// <summary>zap-client.exe stdout JSON wrapper (non-generic).</summary>
    public class ZapResponse
    {
        [JsonProperty("isSuccess")]
        public bool IsSuccess { get; set; }

        [JsonProperty("errorMsg")]
        public string ErrorMsg { get; set; }

        public static ZapResponse OK()
        {
            return new ZapResponse { IsSuccess = true };
        }

        public static ZapResponse NG(string errMsg)
        {
            return new ZapResponse { IsSuccess = false, ErrorMsg = errMsg };
        }

        public static ZapResponse NG(Exception e)
        {
            return new ZapResponse { IsSuccess = false, ErrorMsg = e.Message };
        }
    }

    /// <summary>zap-client.exe stdout JSON wrapper with typed data payload.</summary>
    public class ZapResponse<T> : ZapResponse
    {
        [JsonProperty("data")]
        public T Data { get; set; }

        public static ZapResponse<T> OK(T data)
        {
            return new ZapResponse<T> { IsSuccess = true, Data = data };
        }

        /// <summary>Create a failed response with an error message.</summary>
        public static new ZapResponse<T> NG(string errMsg)
        {
            return new ZapResponse<T> { IsSuccess = false, ErrorMsg = errMsg };
        }

        /// <summary>Create a failed response from an exception.</summary>
        public static new ZapResponse<T> NG(Exception e)
        {
            return new ZapResponse<T> { IsSuccess = false, ErrorMsg = e.Message };
        }
    }
}
