using Newtonsoft.Json;
using System;
using System.Collections.Generic;
using System.Text;

namespace Zap.Client.CSharpSDK.Models
{
    public class ZapResponse
    {
        [JsonProperty("isSuccess")]
        public bool IsSuccess { get; set; }

        [JsonProperty("errorMsg")]
        public string ErrorMsg { get; set; }

        public static ZapResponse OK()
        {
            return new ZapResponse() { IsSuccess = true };
        }

        public static ZapResponse NG(string errMsg)
        {
            return new ZapResponse() { IsSuccess = false, ErrorMsg = errMsg };
        }

        public static ZapResponse NG(Exception e)
        {
            return new ZapResponse() { IsSuccess = false, ErrorMsg = e.Message };
        }
    }
    /// <summary>
    /// zap-client.exe stdout JSON.
    /// </summary>
    public class ZapResponse<T>:ZapResponse
    {
        [JsonProperty("data")]
        public T Data { get; set; }

        public static ZapResponse<T> OK(T data)
        {
            return new ZapResponse<T>() { IsSuccess = true, Data = data };
        }
        public static new ZapResponse<T> NG(string errMsg)
        {
            return new ZapResponse<T>() { IsSuccess = false, ErrorMsg = errMsg };
        }

        public static new ZapResponse<T> NG(Exception e)
        {
            return new ZapResponse<T>() { IsSuccess = false, ErrorMsg = e.Message };
        }
    }
}
