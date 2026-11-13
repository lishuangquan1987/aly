package io.github.lishuangquan1987.alyclient;

import io.github.lishuangquan1987.alyclient.models.CheckSelfUpdateData;
import io.github.lishuangquan1987.alyclient.models.CheckUpdateData;

import java.util.Random;

/**
 * aly-client 的托管生命周期：自更新 → check → download → apply。
 *
 * <p>确认回调在循环线程内同步执行，签名 (version, cancelToken)；返回即确认。
 * 阻塞时应轮询 cancelToken，保证 stop() 能及时退出。
 */
public final class UpdateClient {
    public interface StatusListener {
        void onStatus(String status, String message);
    }

    public interface ErrorListener {
        void onError(String message);
    }

    public interface ConfirmHandler {
        void confirm(String version, CancelToken cancelToken);
    }

    private final API api;
    private final Random random = new Random();
    private final long idlePollIntervalSeconds;

    private volatile boolean running;
    private volatile String status = Status.NONE;
    private volatile String error = "";
    private volatile Thread thread;

    private volatile StatusListener statusListener;
    private volatile ErrorListener errorListener;
    private volatile ConfirmHandler confirmDownload;
    private volatile ConfirmHandler confirmApply;

    public UpdateClient(String path) {
        this(new API(path));
    }

    public UpdateClient(API api) {
        this(api, 300L);
    }

    public UpdateClient(API api, long idlePollIntervalSeconds) {
        this.api = api;
        this.idlePollIntervalSeconds = idlePollIntervalSeconds;
    }

    public void setOnStatusChange(StatusListener listener) {
        this.statusListener = listener;
    }

    public void setOnError(ErrorListener listener) {
        this.errorListener = listener;
    }

    public void setConfirmDownload(ConfirmHandler handler) {
        this.confirmDownload = handler;
    }

    public void setConfirmApply(ConfirmHandler handler) {
        this.confirmApply = handler;
    }

    public String status() {
        return status;
    }

    public String lastError() {
        return error;
    }

    public boolean isRunning() {
        Thread current = thread;
        return current != null && current.isAlive();
    }

    /** 启动后台循环（立即返回）。重复调用无副作用。 */
    public void start() {
        synchronized (this) {
            if (thread != null && thread.isAlive()) {
                return;
            }
            running = true;
            thread = new Thread(new Runnable() {
                @Override
                public void run() {
                    loop();
                }
            }, "aly-update");
            thread.setDaemon(true);
            thread.start();
        }
    }

    /** 停止轮询并等待循环退出；返回是否在超时内退出。 */
    public boolean stop(long timeoutMs) {
        running = false;
        Thread current;
        synchronized (this) {
            current = thread;
        }
        if (current == null) {
            return true;
        }
        try {
            current.join(timeoutMs);
        } catch (InterruptedException ex) {
            Thread.currentThread().interrupt();
        }
        return !current.isAlive();
    }

    private CancelToken token() {
        return new CancelToken() {
            @Override
            public boolean isCancelled() {
                return !running;
            }
        };
    }

    private void loop() {
        trySelfUpdate();
        while (running) {
            Response<CheckUpdateData> check = api.checkUpdate(token());
            if (!running) {
                return;
            }
            if (!check.isSuccess) {
                reportError(check.errorMsg);
                if (sleep(5000L)) {
                    return;
                }
                continue;
            }
            clearError();

            CheckUpdateData data = check.data;
            if (data == null) {
                reportError("check_update returned no data");
                if (sleep(5000L)) {
                    return;
                }
                continue;
            }

            if (!data.hasUpdate) {
                if (!Status.NONE.equals(status)) {
                    setStatus(Status.NONE, "Update status cleared");
                }
                if (sleep(idleDelayMillis())) {
                    return;
                }
                continue;
            }

            boolean force = Boolean.TRUE.equals(data.forceUpdate);
            String version = data.newVersion == null ? "" : data.newVersion;
            // 三态分派（与 C# UpdateClient 一致）：hasUpdate && !needDownloadUpdate
            // 表示本地已下载待应用 / applying 崩溃恢复 / 回滚续跑，直接 applyUpdate，
            // 不下载、不询问（恢复场景自愈）。
            if (data.needDownloadUpdate) {
                String message =
                        version.isEmpty() ? "Found new version" : "Found new version " + version;
                setStatus(Status.DISCOVERED_UPDATE, message);
                if (!force) {
                    callConfirm(confirmDownload, version);
                    if (!running) {
                        return;
                    }
                }

                setStatus(Status.DOWNLOADING_UPDATE, "Downloading update...");
                Response<Void> downloaded = api.downloadUpdate(
                        new ProgressListener() {
                            @Override
                            public void onProgress(DownloadProgressInfo info) {
                                setStatus(
                                        Status.DOWNLOADING_UPDATE,
                                        "Downloading "
                                                + info.file
                                                + "... "
                                                + (int) (info.ratio * 100)
                                                + "%");
                            }
                        },
                        API.DEFAULT_DOWNLOAD_STALL_TIMEOUT_MS,
                        token());
                if (!running) {
                    return;
                }
                if (!downloaded.isSuccess) {
                    if (downloaded.errorMsg != null
                            && downloaded.errorMsg.contains("already at latest version")) {
                        setStatus(Status.NONE, "Update status cleared");
                        if (sleep(idleDelayMillis())) {
                            return;
                        }
                        continue;
                    }
                    reportError(downloaded.errorMsg);
                    if (sleep(1000L)) {
                        return;
                    }
                    continue;
                }
            }

            setStatus(Status.DOWNLOADED_UPDATE, "Update downloaded");
            if (!force) {
                callConfirm(confirmApply, version);
                if (!running) {
                    return;
                }
            }

            setStatus(Status.APPLY_UPDATE, "Applying update...");
            Response<Void> applied = api.applyUpdate(CloseOptions.none());
            if (!applied.isSuccess) {
                reportError("apply spawn failed: " + applied.errorMsg);
            }
            if (sleep(30000L)) {
                return;
            }
        }
    }

    private void trySelfUpdate() {
        Response<CheckSelfUpdateData> check = api.checkSelfUpdate(token());
        if (!running || !check.isSuccess || check.data == null || !check.data.needUpdate) {
            return;
        }
        Response<?> updated = api.selfUpdate(token());
        if (!updated.isSuccess) {
            reportError(updated.errorMsg);
        }
    }

    private void callConfirm(ConfirmHandler handler, String version) {
        if (handler == null) {
            return;
        }
        try {
            handler.confirm(version, token());
        } catch (RuntimeException ex) {
            reportError("confirm handler error: " + ex.getMessage());
        }
    }

    private void setStatus(String nextStatus, String message) {
        status = nextStatus;
        StatusListener listener = statusListener;
        if (listener != null) {
            try {
                listener.onStatus(nextStatus, message);
            } catch (RuntimeException ignored) {
                // 回调异常不得打断循环
            }
        }
    }

    private synchronized void reportError(String message) {
        String next = message == null ? "" : message;
        if (next.equals(error)) {
            return;
        }
        error = next;
        ErrorListener listener = errorListener;
        if (listener != null) {
            try {
                listener.onError(next);
            } catch (RuntimeException ignored) {
                // 回调异常不得打断循环
            }
        }
    }

    private synchronized void clearError() {
        if (error.isEmpty()) {
            return;
        }
        error = "";
        ErrorListener listener = errorListener;
        if (listener != null) {
            try {
                listener.onError("");
            } catch (RuntimeException ignored) {
                // ignore
            }
        }
    }

    /** 返回 true 表示循环应退出。 */
    private boolean sleep(long millis) {
        long end = System.currentTimeMillis() + millis;
        while (running && System.currentTimeMillis() < end) {
            long remaining = end - System.currentTimeMillis();
            try {
                Thread.sleep(Math.min(100L, Math.max(1L, remaining)));
            } catch (InterruptedException ex) {
                Thread.currentThread().interrupt();
                return true;
            }
        }
        return !running;
    }

    private long idleDelayMillis() {
        long seconds = idlePollIntervalSeconds;
        if (seconds <= 0) {
            seconds = 300L;
        }
        if (seconds < 60L) {
            seconds = 60L;
        }
        return (seconds + random.nextInt(61)) * 1000L;
    }
}