package io.github.lishuangquan1987.alyclient;

/** 所有命令的统一响应；业务失败不会抛异常。 */
public final class Response<T> {
    public final boolean isSuccess;
    public final String errorMsg;
    public final T data;

    public Response(boolean isSuccess, String errorMsg, T data) {
        this.isSuccess = isSuccess;
        this.errorMsg = errorMsg;
        this.data = data;
    }

    public static <T> Response<T> ok(T data) {
        return new Response<T>(true, "", data);
    }

    public static <T> Response<T> ng(String errorMsg) {
        return new Response<T>(false, errorMsg, null);
    }
}