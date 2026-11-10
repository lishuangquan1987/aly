package io.github.lishuangquan1987.alyclient;

/** 下载进度回调。 */
@FunctionalInterface
public interface ProgressListener {
    void onProgress(DownloadProgressInfo info);
}