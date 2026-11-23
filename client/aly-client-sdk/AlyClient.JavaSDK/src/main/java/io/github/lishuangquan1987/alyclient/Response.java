package io.github.lishuangquan1987.alyclient;

import java.util.Map;

/** 所有命令的统一响应；业务失败不会抛异常。 */
public final class Response<T> {
    public final boolean isSuccess;
    public final String errorMsg;
    public final T data;
    /** download_update 失败终态的原因机器码（协议 #13）：already_latest / partial_failure / materialize_failed / version_state_failed。机器判定优先于 errorMsg 文案匹配。 */
    public final String reason;

    public Response(boolean isSuccess, String errorMsg, T data) {
        this(isSuccess, errorMsg, data, null);
    }

    public Response(boolean isSuccess, String errorMsg, T data, String reason) {
        this.isSuccess = isSuccess;
        this.errorMsg = errorMsg;
        this.data = data;
        this.reason = reason;
    }

    public static <T> Response<T> ok(T data) {
        return new Response<T>(true, "", data);
    }

    public static <T> Response<T> ng(String errorMsg) {
        return new Response<T>(false, errorMsg, null);
    }

    /** 失败响应 + reason 机器码（协议 #13）。 */
    public static <T> Response<T> ng(String errorMsg, String reason) {
        return new Response<T>(false, errorMsg, null, reason);
    }

    /** 便捷判定：下载失败是否"已是最新"（#13：reason 优先，文案匹配兼容回退）。 */
    public boolean isAlreadyLatest() {
        if ("already_latest".equals(reason)) {
            return true;
        }
        return reason == null && errorMsg != null && errorMsg.contains("already at latest version");
    }
}