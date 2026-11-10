package io.github.lishuangquan1987.alyclient;

import io.github.lishuangquan1987.alyclient.models.CheckDiffData;
import io.github.lishuangquan1987.alyclient.models.CheckSelfUpdateData;
import io.github.lishuangquan1987.alyclient.models.CheckUpdateData;
import io.github.lishuangquan1987.alyclient.models.CleanData;
import io.github.lishuangquan1987.alyclient.models.DownloadProgress;
import io.github.lishuangquan1987.alyclient.models.RollbackData;
import io.github.lishuangquan1987.alyclient.models.RollbackListData;
import io.github.lishuangquan1987.alyclient.models.SelfUpdateData;

import java.io.BufferedReader;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.nio.charset.StandardCharsets;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.Collections;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.BlockingQueue;
import java.util.concurrent.LinkedBlockingQueue;
import java.util.concurrent.TimeUnit;
import java.util.function.Function;

/** 封装一个 aly-client 可执行文件。 */
public final class API {
    public static final long DEFAULT_SHORT_TIMEOUT_MS = 30000L;
    public static final long DEFAULT_DOWNLOAD_STALL_TIMEOUT_MS = 1800000L;
    public static final long ROLLBACK_TIMEOUT_MS = 600000L;

    private static final String STREAM_END = "\u0000EOF";

    private final Path clientPath;
    private final Map<String, String> extraEnv;

    public API(String path) {
        this(path, Collections.<String, String>emptyMap());
    }

    public API(String path, Map<String, String> env) {
        String target = (path == null || path.isEmpty()) ? defaultPath().toString() : path;
        this.clientPath = Paths.get(target).toAbsolutePath().normalize();
        this.extraEnv = env == null
                ? Collections.<String, String>emptyMap()
                : new HashMap<String, String>(env);
    }

    public Path path() {
        return clientPath;
    }

    /** 默认更新器路径：<user.dir>/../UpdateFolder/aly-client[.exe]。 */
    public static Path defaultPath() {
        String name = isWindows() ? "aly-client.exe" : "aly-client";
        return Paths.get(System.getProperty("user.dir"))
                .resolve("..")
                .resolve("UpdateFolder")
                .resolve(name)
                .toAbsolutePath()
                .normalize();
    }

    private static boolean isWindows() {
        return System.getProperty("os.name", "").toLowerCase().contains("win");
    }

    public Response<CheckUpdateData> checkUpdate() {
        return checkUpdate(CancelToken.NEVER);
    }

    public Response<CheckUpdateData> checkUpdate(CancelToken token) {
        return runShort("check_update", DEFAULT_SHORT_TIMEOUT_MS, token, CheckUpdateData::fromMap);
    }

    public Response<CheckSelfUpdateData> checkSelfUpdate() {
        return checkSelfUpdate(CancelToken.NEVER);
    }

    public Response<CheckSelfUpdateData> checkSelfUpdate(CancelToken token) {
        return runShort(
                "check_self_update", DEFAULT_SHORT_TIMEOUT_MS, token, CheckSelfUpdateData::fromMap);
    }

    public Response<SelfUpdateData> selfUpdate() {
        return selfUpdate(CancelToken.NEVER);
    }

    public Response<SelfUpdateData> selfUpdate(CancelToken token) {
        return runShort("self_update", DEFAULT_SHORT_TIMEOUT_MS, token, SelfUpdateData::fromMap);
    }

    public Response<RollbackListData> listRollbackVersions() {
        return runShort(
                "list_rollback_versions",
                DEFAULT_SHORT_TIMEOUT_MS,
                CancelToken.NEVER,
                RollbackListData::fromMap);
    }

    public Response<CheckDiffData> checkDiff() {
        return runShort("check_diff", DEFAULT_SHORT_TIMEOUT_MS, CancelToken.NEVER, CheckDiffData::fromMap);
    }

    public Response<CleanData> clean() {
        return clean(null, false);
    }

    public Response<CleanData> clean(Integer keep, boolean all) {
        List<String> args = new ArrayList<String>();
        args.add("clean");
        if (all) {
            args.add("--all");
        } else if (keep != null && keep.intValue() > 0) {
            args.add("--keep");
            args.add(String.valueOf(keep));
        }
        return parse(
                runCollect(args, DEFAULT_SHORT_TIMEOUT_MS, CancelToken.NEVER),
                DEFAULT_SHORT_TIMEOUT_MS,
                CleanData::fromMap);
    }

    public Response<RollbackData> rollback(String version) {
        return rollback(version, CloseOptions.none());
    }

    public Response<RollbackData> rollback(String version, CloseOptions options) {
        List<String> args = new ArrayList<String>();
        args.add("rollback");
        args.add("--version");
        args.add(version);
        appendCloseArgs(args, options);
        return parse(
                runCollect(args, ROLLBACK_TIMEOUT_MS, CancelToken.NEVER),
                ROLLBACK_TIMEOUT_MS,
                RollbackData::fromMap);
    }

    public Response<Void> downloadUpdate() {
        return downloadUpdate(null, DEFAULT_DOWNLOAD_STALL_TIMEOUT_MS, CancelToken.NEVER);
    }

    /**
     * 下载差异文件。没有总时长上限，只有停流看门狗；进度只统计 DONE 行。
     */
    public Response<Void> downloadUpdate(
            ProgressListener listener, long stallTimeoutMs, CancelToken token) {
        Process process;
        try {
            process = start(Collections.singletonList("download_update"));
        } catch (IOException ex) {
            return Response.ng(ex.getMessage());
        }

        final BlockingQueue<String> lines = new LinkedBlockingQueue<String>();
        final Collector stderr = new Collector(process.getErrorStream());
        Thread errThread = new Thread(stderr, "aly-client-stderr");
        errThread.setDaemon(true);
        errThread.start();

        Thread reader = new Thread(new Runnable() {
            @Override
            public void run() {
                try (BufferedReader br =
                        new BufferedReader(
                                new InputStreamReader(process.getInputStream(), StandardCharsets.UTF_8))) {
                    String line;
                    while ((line = br.readLine()) != null) {
                        lines.offer(line);
                    }
                } catch (IOException ignored) {
                    // 进程被 kill 时读流会异常，按 EOF 处理
                } finally {
                    lines.offer(STREAM_END);
                }
            }
        }, "aly-client-download");
        reader.setDaemon(true);
        reader.start();

        long stallMs = stallTimeoutMs <= 0 ? DEFAULT_DOWNLOAD_STALL_TIMEOUT_MS : stallTimeoutMs;
        long lastOutput = System.currentTimeMillis();

        while (true) {
            if (token != null && token.isCancelled()) {
                kill(process);
                return Response.ng("download canceled");
            }
            String line;
            try {
                line = lines.poll(200, TimeUnit.MILLISECONDS);
            } catch (InterruptedException ex) {
                Thread.currentThread().interrupt();
                kill(process);
                return Response.ng("download interrupted");
            }
            if (line == null) {
                if (System.currentTimeMillis() - lastOutput > stallMs) {
                    kill(process);
                    return Response.ng(
                            "download stalled: no output for "
                                    + (stallMs / 1000)
                                    + "s, process killed");
                }
                continue;
            }
            if (STREAM_END.equals(line)) {
                waitQuietly(process);
                joinQuietly(errThread);
                String errText = stderr.text().trim();
                return Response.ng(
                        errText.isEmpty()
                                ? "download interrupted (aly-client exited before finish line)"
                                : errText);
            }
            lastOutput = System.currentTimeMillis();
            String trimmed = line.trim();
            if (trimmed.isEmpty()) {
                continue;
            }

            Map<String, Object> payload;
            try {
                payload = Json.asObject(Json.parse(trimmed));
            } catch (RuntimeException ex) {
                kill(process);
                return Response.ng("invalid progress line: " + ex.getMessage());
            }
            if (!Json.bool(payload, "isSuccess", false)) {
                kill(process);
                String message = Json.str(payload, "errorMsg");
                return Response.ng(message.isEmpty() ? "download failed" : message);
            }
            Object data = payload.get("data");
            if (data == null) {
                return Response.ok(null);
            }

            Map<String, Object> progress;
            try {
                progress = Json.asObject(data);
            } catch (RuntimeException ex) {
                kill(process);
                return Response.ng("invalid progress data: " + ex.getMessage());
            }
            long total = Json.lng(progress, "total", 0L);
            String status = Json.str(progress, "status");
            if (DownloadProgress.STATUS_DONE.equals(status) && total > 0 && listener != null) {
                long index = Json.lng(progress, "index", 0L);
                try {
                    listener.onProgress(new DownloadProgressInfo(
                            Json.str(progress, "file"),
                            total == 0 ? 0.0 : ((double) index / (double) total),
                            status,
                            index,
                            total,
                            Json.lng(progress, "downloaded_bytes", 0L),
                            Json.lng(progress, "total_bytes", 0L),
                            Json.lng(progress, "speed_bps", 0L)));
                } catch (RuntimeException ignored) {
                    // 进度回调异常不得打断下载
                }
            }
        }
    }

    /** 拉起独立更新进程后立即返回，不等待。 */
    public Response<Void> applyUpdate(CloseOptions options) {
        List<String> args = new ArrayList<String>();
        args.add("apply_update");
        appendCloseArgs(args, options);
        try {
            start(args);
        } catch (IOException ex) {
            return Response.ng(ex.getMessage());
        }
        return Response.ok(null);
    }

    private <T> Response<T> runShort(
            String command,
            long timeoutMs,
            CancelToken token,
            Function<Map<String, Object>, T> mapper) {
        return parse(
                runCollect(Collections.singletonList(command), timeoutMs, token), timeoutMs, mapper);
    }

    private <T> Response<T> parse(
            RunResult result, long timeoutMs, Function<Map<String, Object>, T> mapper) {
        if (result.startError != null) {
            return Response.ng(result.startError);
        }
        if (result.canceled) {
            return Response.ng("aly-client call canceled");
        }
        if (result.timedOut) {
            return Response.ng(
                    "aly-client timed out after " + (timeoutMs / 1000) + "s and was killed");
        }

        String text = result.stdoutText().trim();
        if (text.isEmpty()) {
            String errText = result.stderrText().trim();
            if (result.exitCode != 0) {
                return Response.ng(
                        errText.isEmpty()
                                ? "aly-client exited with code "
                                        + result.exitCode
                                        + " (may require admin)"
                                : errText);
            }
            return Response.ng("aly-client returned no output (may require admin)");
        }

        Map<String, Object> payload;
        try {
            payload = Json.asObject(Json.parse(text));
        } catch (RuntimeException ex) {
            return Response.ng(ex.getMessage());
        }
        Object data = payload.get("data");
        T typed = null;
        if (data != null && mapper != null) {
            typed = mapper.apply(Json.asObject(data));
        }
        return new Response<T>(
                Json.bool(payload, "isSuccess", false), Json.str(payload, "errorMsg"), typed);
    }

    private Process start(List<String> args) throws IOException {
        List<String> command = new ArrayList<String>();
        command.add(clientPath.toString());
        command.addAll(args);
        ProcessBuilder builder = new ProcessBuilder(command);
        Path parent = clientPath.getParent();
        if (parent != null) {
            builder.directory(parent.toFile());
        }
        builder.environment().putAll(extraEnv);
        return builder.start();
    }

    private RunResult runCollect(List<String> args, long timeoutMs, CancelToken token) {
        Process process;
        try {
            process = start(args);
        } catch (IOException ex) {
            return new RunResult(null, null, -1, false, false, ex.getMessage());
        }

        Collector stdout = new Collector(process.getInputStream());
        Collector stderr = new Collector(process.getErrorStream());
        Thread outThread = new Thread(stdout, "aly-client-stdout");
        Thread errThread = new Thread(stderr, "aly-client-stderr");
        outThread.setDaemon(true);
        errThread.setDaemon(true);
        outThread.start();
        errThread.start();

        long deadline = System.currentTimeMillis() + timeoutMs;
        boolean timedOut = false;
        boolean canceled = false;
        while (process.isAlive()) {
            if (token != null && token.isCancelled()) {
                canceled = true;
                kill(process);
                break;
            }
            if (timeoutMs > 0 && System.currentTimeMillis() > deadline) {
                timedOut = true;
                kill(process);
                break;
            }
            sleepQuietly(20L);
        }

        int exitCode = waitQuietly(process);
        joinQuietly(outThread);
        joinQuietly(errThread);
        return new RunResult(
                stdout.bytes(), stderr.bytes(), exitCode, timedOut, canceled, null);
    }

    private static void appendCloseArgs(List<String> args, CloseOptions options) {
        if (options == null) {
            return;
        }
        String names = options.mustCloseProcessNames == null
                ? ""
                : options.mustCloseProcessNames.trim();
        if (!names.isEmpty()) {
            args.add("--must-close-process-name");
            args.add(names);
        }
        if (options.closeTimeoutSeconds != 0 && options.closeTimeoutSeconds != 30) {
            args.add("--close-timeout");
            args.add(String.valueOf(options.closeTimeoutSeconds));
        }
    }

    private static void kill(Process process) {
        try {
            process.destroyForcibly();
        } catch (RuntimeException ignored) {
            // 进程已退出
        }
    }

    private static int waitQuietly(Process process) {
        try {
            return process.waitFor();
        } catch (InterruptedException ex) {
            Thread.currentThread().interrupt();
            return -1;
        }
    }

    private static void joinQuietly(Thread thread) {
        try {
            thread.join(2000L);
        } catch (InterruptedException ex) {
            Thread.currentThread().interrupt();
        }
    }

    private static void sleepQuietly(long millis) {
        try {
            Thread.sleep(millis);
        } catch (InterruptedException ex) {
            Thread.currentThread().interrupt();
        }
    }

    private static final class Collector implements Runnable {
        private final InputStream input;
        private final ByteArrayOutputStream buffer = new ByteArrayOutputStream();

        Collector(InputStream input) {
            this.input = input;
        }

        @Override
        public void run() {
            byte[] chunk = new byte[4096];
            int read;
            try {
                while ((read = input.read(chunk)) >= 0) {
                    buffer.write(chunk, 0, read);
                }
            } catch (IOException ignored) {
                // 进程退出/被 kill 时读流可能异常
            } finally {
                try {
                    input.close();
                } catch (IOException ignored) {
                    // ignore
                }
            }
        }

        byte[] bytes() {
            synchronized (buffer) {
                return buffer.toByteArray();
            }
        }

        String text() {
            return new String(bytes(), StandardCharsets.UTF_8);
        }
    }

    private static final class RunResult {
        final byte[] stdout;
        final byte[] stderr;
        final int exitCode;
        final boolean timedOut;
        final boolean canceled;
        final String startError;

        RunResult(
                byte[] stdout,
                byte[] stderr,
                int exitCode,
                boolean timedOut,
                boolean canceled,
                String startError) {
            this.stdout = stdout;
            this.stderr = stderr;
            this.exitCode = exitCode;
            this.timedOut = timedOut;
            this.canceled = canceled;
            this.startError = startError;
        }

        String stdoutText() {
            return new String(stdout == null ? new byte[0] : stdout, StandardCharsets.UTF_8);
        }

        String stderrText() {
            return new String(stderr == null ? new byte[0] : stderr, StandardCharsets.UTF_8);
        }
    }
}