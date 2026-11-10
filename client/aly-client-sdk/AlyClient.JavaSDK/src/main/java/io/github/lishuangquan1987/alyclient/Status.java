package io.github.lishuangquan1987.alyclient;

/** 更新生命周期阶段。 */
public final class Status {
    public static final String NONE = "none";
    public static final String DISCOVERED_UPDATE = "discoveredUpdate";
    public static final String DOWNLOADING_UPDATE = "downloadingUpdate";
    public static final String DOWNLOADED_UPDATE = "downloadedUpdate";
    public static final String APPLY_UPDATE = "applyUpdate";

    private Status() {
    }
}