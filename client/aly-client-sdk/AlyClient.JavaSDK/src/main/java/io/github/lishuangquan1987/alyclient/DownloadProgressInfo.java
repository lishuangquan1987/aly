package io.github.lishuangquan1987.alyclient;

/** 下载进度（文件计数粒度；字节字段可选）。 */
public final class DownloadProgressInfo {
    public final String file;
    public final double ratio;
    public final String status;
    public final long index;
    public final long total;
    public final long downloadedBytes;
    public final long totalBytes;
    public final long speedBps;

    public DownloadProgressInfo(
            String file,
            double ratio,
            String status,
            long index,
            long total,
            long downloadedBytes,
            long totalBytes,
            long speedBps) {
        this.file = file;
        this.ratio = ratio;
        this.status = status;
        this.index = index;
        this.total = total;
        this.downloadedBytes = downloadedBytes;
        this.totalBytes = totalBytes;
        this.speedBps = speedBps;
    }
}