package io.github.lishuangquan1987.alyclient;

/** apply_update / rollback 的进程关闭参数。 */
public final class CloseOptions {
    public final String mustCloseProcessNames;
    public final int closeTimeoutSeconds;

    public CloseOptions(String mustCloseProcessNames, int closeTimeoutSeconds) {
        this.mustCloseProcessNames = mustCloseProcessNames;
        this.closeTimeoutSeconds = closeTimeoutSeconds;
    }

    public static CloseOptions none() {
        return new CloseOptions(null, 30);
    }
}