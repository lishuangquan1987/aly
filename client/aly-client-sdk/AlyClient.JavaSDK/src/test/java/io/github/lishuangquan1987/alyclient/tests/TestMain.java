package io.github.lishuangquan1987.alyclient.tests;

import io.github.lishuangquan1987.alyclient.API;
import io.github.lishuangquan1987.alyclient.CancelToken;
import io.github.lishuangquan1987.alyclient.CloseOptions;
import io.github.lishuangquan1987.alyclient.DownloadProgressInfo;
import io.github.lishuangquan1987.alyclient.Response;
import io.github.lishuangquan1987.alyclient.Status;
import io.github.lishuangquan1987.alyclient.UpdateClient;
import io.github.lishuangquan1987.alyclient.models.CheckDiffData;
import io.github.lishuangquan1987.alyclient.models.CheckSelfUpdateData;
import io.github.lishuangquan1987.alyclient.models.CheckUpdateData;
import io.github.lishuangquan1987.alyclient.models.CleanData;
import io.github.lishuangquan1987.alyclient.models.RollbackData;
import io.github.lishuangquan1987.alyclient.models.RollbackListData;
import io.github.lishuangquan1987.alyclient.models.SelfUpdateData;

import java.io.FileDescriptor;
import java.io.FileOutputStream;
import java.io.PrintStream;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.Collections;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/** 用假客户端回放 fixtures 的 Java 协议级一致性用例。 */
public final class TestMain {
    private static Path fake;
    private static Path scenarios;
    private static int pass;
    private static int fail;

    private interface CaseBody {
        String run() throws Exception;
    }

    public static void main(String[] args) throws Exception {
        try {
            System.setOut(new PrintStream(new FileOutputStream(FileDescriptor.out), true, "UTF-8"));
        } catch (Exception ignored) {
            // 受限环境保持默认编码
        }
        if (args.length < 2) {
            System.out.println("用法: TestMain <fake-aly-client> <scenariosDir>");
            System.exit(2);
        }
        fake = Paths.get(args[0]).toAbsolutePath();
        scenarios = Paths.get(args[1]).toAbsolutePath();

        check("check_update: 发现新版本字段完整", new CaseBody() {
            @Override
            public String run() {
                Response<CheckUpdateData> response = api("update_available").checkUpdate();
                if (!response.isSuccess) {
                    return "NG: " + response.errorMsg;
                }
                CheckUpdateData data = response.data;
                if (!data.hasUpdate || !data.needDownloadUpdate) {
                    return "has_update/need_download_update 不符";
                }
                if (!"1.0.1".equals(data.newVersion)) {
                    return "new_version=" + data.newVersion;
                }
                if (Boolean.TRUE.equals(data.forceUpdate)) {
                    return "force_update 应为 false";
                }
                if (data.releaseNotes == null || data.releaseNotes.size() != 2) {
                    return "release_notes 数量不符";
                }
                return null;
            }
        });

        check("check_update: 已是最新 / 离线", new CaseBody() {
            @Override
            public String run() {
                for (String scenario : new String[] {"already_latest", "offline"}) {
                    Response<CheckUpdateData> response = api(scenario).checkUpdate();
                    if (!response.isSuccess) {
                        return scenario + " NG: " + response.errorMsg;
                    }
                    if (response.data.hasUpdate) {
                        return scenario + " has_update=true";
                    }
                }
                return null;
            }
        });

        check("download_update: 进度回调与完成行", new CaseBody() {
            @Override
            public String run() {
                final List<DownloadProgressInfo> progress = new ArrayList<DownloadProgressInfo>();
                Response<Void> response = api("update_available").downloadUpdate(
                        progress::add, API.DEFAULT_DOWNLOAD_STALL_TIMEOUT_MS, CancelToken.NEVER);
                if (!response.isSuccess) {
                    return "NG: " + response.errorMsg;
                }
                if (progress.size() != 1) {
                    return "progress=" + progress.size();
                }
                DownloadProgressInfo info = progress.get(0);
                if (!"app.exe".equals(info.file) || info.ratio != 1.0 || !"DONE".equals(info.status)) {
                    return "progress info 不符";
                }
                if (info.downloadedBytes != 1000L) {
                    return "downloadedBytes=" + info.downloadedBytes;
                }
                return null;
            }
        });

        check("download_update: 部分失败透传", new CaseBody() {
            @Override
            public String run() {
                Response<Void> response = api("download_partial_failure").downloadUpdate();
                if (response.isSuccess) {
                    return "部分失败应为 NG";
                }
                if (response.errorMsg == null || !response.errorMsg.contains("checksum mismatch")) {
                    return "errorMsg=" + response.errorMsg;
                }
                return null;
            }
        });

        check("download_update: 停流看门狗", new CaseBody() {
            @Override
            public String run() throws Exception {
                long started = System.currentTimeMillis();
                Response<Void> response = api("download_stall").downloadUpdate(null, 1000L, CancelToken.NEVER);
                long elapsed = System.currentTimeMillis() - started;
                if (response.isSuccess) {
                    return "停流应 NG";
                }
                if (response.errorMsg == null || !response.errorMsg.contains("stalled")) {
                    return "errorMsg=" + response.errorMsg;
                }
                if (elapsed > 4000L) {
                    return "watchdog elapsed=" + elapsed;
                }
                return null;
            }
        });

        check("apply_update: 拉起即返回", new CaseBody() {
            @Override
            public String run() {
                long started = System.currentTimeMillis();
                Response<Void> response = api("update_available")
                        .applyUpdate(new CloseOptions("A,B", 45));
                long elapsed = System.currentTimeMillis() - started;
                if (!response.isSuccess) {
                    return "NG: " + response.errorMsg;
                }
                if (elapsed > 2000L) {
                    return "apply 等待了 " + elapsed + "ms";
                }
                return null;
            }
        });

        check("self_update: 检测与替换", new CaseBody() {
            @Override
            public String run() {
                API api = api("self_update");
                Response<CheckSelfUpdateData> check = api.checkSelfUpdate();
                if (!check.isSuccess || check.data == null || !check.data.needUpdate) {
                    return "check_self_update 不符";
                }
                Response<SelfUpdateData> updated = api.selfUpdate();
                if (!updated.isSuccess || updated.data == null || !updated.data.updated) {
                    return "self_update 不符: " + updated.errorMsg;
                }
                if (!"aly-client.exe.old".equals(updated.data.oldPath)) {
                    return "old_path=" + updated.data.oldPath;
                }
                return null;
            }
        });

        check("rollback: 列表与回滚", new CaseBody() {
            @Override
            public String run() {
                API api = api("rollback");
                Response<RollbackListData> list = api.listRollbackVersions();
                if (!list.isSuccess || list.data == null || list.data.versions.size() != 2) {
                    return "versions 不符";
                }
                Response<RollbackData> rolled = api.rollback("1.0.0");
                if (!rolled.isSuccess || rolled.data == null || !"1.0.0".equals(rolled.data.version)) {
                    return "rollback 不符: " + rolled.errorMsg;
                }
                return null;
            }
        });

        check("check_diff / clean", new CaseBody() {
            @Override
            public String run() {
                Response<CheckDiffData> diff = api("check_diff").checkDiff();
                if (!diff.isSuccess
                        || diff.data == null
                        || diff.data.files.size() != 1
                        || !"app.exe".equals(diff.data.files.get(0).path)) {
                    return "check_diff 不符";
                }
                Response<CleanData> cleaned = api("clean_all").clean(null, true);
                if (!cleaned.isSuccess || cleaned.data == null || !"all".equals(cleaned.data.mode)) {
                    return "clean 不符";
                }
                return null;
            }
        });

        check("缺失更新器: 返回失败而不是抛异常", new CaseBody() {
            @Override
            public String run() {
                Path missing = Paths.get(System.getProperty("java.io.tmpdir"))
                        .resolve("aly-java-missing")
                        .resolve("aly-client.exe");
                Response<CheckUpdateData> response = new API(missing.toString()).checkUpdate();
                if (response.isSuccess) {
                    return "缺失更新器应为 NG";
                }
                if (response.errorMsg == null || response.errorMsg.isEmpty()) {
                    return "errorMsg 为空";
                }
                return null;
            }
        });

        check("defaultPath 指向 UpdateFolder", new CaseBody() {
            @Override
            public String run() {
                if (!API.defaultPath().toString().contains("UpdateFolder")) {
                    return "defaultPath=" + API.defaultPath();
                }
                return null;
            }
        });

        check("托管循环: 强制更新完整链路", new CaseBody() {
            @Override
            public String run() throws Exception {
                final List<String> statuses = Collections.synchronizedList(new ArrayList<String>());
                UpdateClient client = new UpdateClient(api("force_update"));
                client.setOnStatusChange(new UpdateClient.StatusListener() {
                    @Override
                    public void onStatus(String status, String message) {
                        statuses.add(status);
                    }
                });
                client.start();
                waitForStatus(client, Status.APPLY_UPDATE, 5000L);
                if (!client.stop(5000L)) {
                    return "stop 超时";
                }
                if (statuses.isEmpty() || !Status.DISCOVERED_UPDATE.equals(statuses.get(0))) {
                    return "statuses=" + statuses;
                }
                if (!statuses.contains(Status.DOWNLOADING_UPDATE)
                        || !statuses.contains(Status.DOWNLOADED_UPDATE)) {
                    return "statuses=" + statuses;
                }
                if (!Status.APPLY_UPDATE.equals(statuses.get(statuses.size() - 1))) {
                    return "last status=" + statuses.get(statuses.size() - 1);
                }
                if (!client.lastError().isEmpty()) {
                    return "lastError=" + client.lastError();
                }
                return null;
            }
        });

        check("托管循环: 非强制更新等待确认", new CaseBody() {
            @Override
            public String run() throws Exception {
                final List<String> downloads = Collections.synchronizedList(new ArrayList<String>());
                final List<String> applies = Collections.synchronizedList(new ArrayList<String>());
                UpdateClient client = new UpdateClient(api("update_available"));
                client.setConfirmDownload(new UpdateClient.ConfirmHandler() {
                    @Override
                    public void confirm(String version, CancelToken token) {
                        downloads.add(version);
                    }
                });
                client.setConfirmApply(new UpdateClient.ConfirmHandler() {
                    @Override
                    public void confirm(String version, CancelToken token) {
                        applies.add(version);
                    }
                });
                client.start();
                waitForStatus(client, Status.APPLY_UPDATE, 5000L);
                if (!client.stop(5000L)) {
                    return "stop 超时";
                }
                if (!downloads.equals(Collections.singletonList("1.0.1"))) {
                    return "download confirms=" + downloads;
                }
                if (!applies.equals(Collections.singletonList("1.0.1"))) {
                    return "apply confirms=" + applies;
                }
                return null;
            }
        });

        check("托管循环: 离线保持空闲", new CaseBody() {
            @Override
            public String run() throws Exception {
                UpdateClient client = new UpdateClient(api("offline"));
                final List<String> asked = Collections.synchronizedList(new ArrayList<String>());
                client.setConfirmDownload(new UpdateClient.ConfirmHandler() {
                    @Override
                    public void confirm(String version, CancelToken token) {
                        asked.add(version);
                    }
                });
                client.start();
                Thread.sleep(500L);
                if (!Status.NONE.equals(client.status())) {
                    return "status=" + client.status();
                }
                client.stop(5000L);
                if (!asked.isEmpty()) {
                    return "confirm 不应被调用: " + asked;
                }
                return null;
            }
        });

        check("托管循环: stop 中断阻塞确认", new CaseBody() {
            @Override
            public String run() throws Exception {
                UpdateClient client = new UpdateClient(api("update_available"));
                client.setConfirmDownload(new UpdateClient.ConfirmHandler() {
                    @Override
                    public void confirm(String version, CancelToken token) {
                        while (!token.isCancelled()) {
                            try {
                                Thread.sleep(10L);
                            } catch (InterruptedException ex) {
                                Thread.currentThread().interrupt();
                                return;
                            }
                        }
                    }
                });
                client.start();
                waitForStatus(client, Status.DISCOVERED_UPDATE, 5000L);
                long started = System.currentTimeMillis();
                boolean stopped = client.stop(5000L);
                long elapsed = System.currentTimeMillis() - started;
                if (!stopped) {
                    return "stop 未在超时内完成";
                }
                if (elapsed > 2000L) {
                    return "stop 耗时 " + elapsed + "ms";
                }
                return null;
            }
        });

        check("托管循环: stop 幂等", new CaseBody() {
            @Override
            public String run() throws Exception {
                UpdateClient client = new UpdateClient(api("offline"));
                client.start();
                Thread.sleep(100L);
                if (!client.stop(5000L) || !client.stop(5000L)) {
                    return "stop 返回 false";
                }
                if (client.isRunning()) {
                    return "isRunning=true";
                }
                return null;
            }
        });

        System.out.println("SUMMARY: PASS=" + pass + " FAIL=" + fail);
        System.exit(fail == 0 ? 0 : 1);
    }

    private static API api(String scenario) {
        Map<String, String> env = new HashMap<String, String>();
        env.put("ALY_FAKE_SCENARIO", scenarios.resolve(scenario + ".json").toString());
        return new API(fake.toString(), env);
    }

    private static void waitForStatus(UpdateClient client, String want, long timeoutMs)
            throws InterruptedException {
        long deadline = System.currentTimeMillis() + timeoutMs;
        while (System.currentTimeMillis() < deadline) {
            if (want.equals(client.status())) {
                return;
            }
            Thread.sleep(20L);
        }
        throw new IllegalStateException(
                "waitForStatus timeout, last=" + client.status() + ", error=" + client.lastError());
    }

    private static void check(String name, CaseBody body) {
        String error;
        try {
            error = body.run();
        } catch (Throwable ex) {
            error = ex.toString();
        }
        if (error == null) {
            pass++;
            System.out.println("PASS: " + name);
        } else {
            fail++;
            System.out.println("FAIL: " + name + " —— " + error);
        }
    }

    private TestMain() {
    }
}