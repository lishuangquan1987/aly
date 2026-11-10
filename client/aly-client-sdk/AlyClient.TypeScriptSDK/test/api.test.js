const test = require("node:test");
const assert = require("node:assert/strict");
const {
  checkUpdate,
  checkSelfUpdate,
  selfUpdate,
  listRollbackVersions,
  rollback,
  checkDiff,
  clean,
  downloadUpdate,
  applyUpdate,
  defaultClientPath,
} = require("../dist");
const { fakeClient, withScenario, tempScenario } = require("./helpers");

const client = fakeClient();

test("checkUpdate parses an available update", async () => {
  await withScenario("update_available", async () => {
    const response = await checkUpdate(client);
    assert.equal(response.isSuccess, true);
    assert.equal(response.data.has_update, true);
    assert.equal(response.data.need_download_update, true);
    assert.equal(response.data.new_version, "1.0.1");
    assert.equal(response.data.force_update, false);
    assert.deepEqual(response.data.release_notes, ["修复登录问题", "优化启动速度"]);
  });
});

test("checkUpdate reports already-latest as no update", async () => {
  await withScenario("already_latest", async () => {
    const response = await checkUpdate(client);
    assert.equal(response.isSuccess, true);
    assert.equal(response.data.has_update, false);
    assert.equal(response.data.need_download_update, false);
  });
});

test("checkUpdate reports offline as no update", async () => {
  await withScenario("offline", async () => {
    const response = await checkUpdate(client);
    assert.equal(response.isSuccess, true);
    assert.equal(response.data.has_update, false);
  });
});

test("downloadUpdate streams DONE progress and finishes on data:null", async () => {
  await withScenario("update_available", async () => {
    const progress = [];
    const response = await downloadUpdate(client, (p) => progress.push(p));
    assert.equal(response.isSuccess, true);
    assert.equal(response.data, null);
    assert.equal(progress.length, 1);
    assert.equal(progress[0].file, "app.exe");
    assert.equal(progress[0].ratio, 1);
    assert.equal(progress[0].status, "DONE");
    assert.equal(progress[0].downloadedBytes, 1000);
  });
});

test("downloadUpdate surfaces partial failure", async () => {
  await withScenario("download_partial_failure", async () => {
    const progress = [];
    const response = await downloadUpdate(client, (p) => progress.push(p));
    assert.equal(response.isSuccess, false);
    assert.match(response.errorMsg, /下载完成但有 1 个文件失败/);
    assert.equal(progress.length, 0);
  });
});

test("downloadUpdate kills a stalled stream", async () => {
  const path = tempScenario({
    name: "stall",
    commands: {
      download_update: {
        lines: [
          { isSuccess: true, errorMsg: "", data: { index: 0, total: 1, file: "", status: "BEGIN", file_size: 0 } },
          { isSuccess: true, errorMsg: "", data: null },
        ],
        line_delay_ms: 5000,
      },
    },
  });
  await withScenario(path, async () => {
    const started = Date.now();
    const response = await downloadUpdate(client, undefined, { stallTimeoutSeconds: 1 });
    assert.equal(response.isSuccess, false);
    assert.match(response.errorMsg, /download stalled/);
    assert.ok(Date.now() - started < 3000, "watchdog should kill within ~1s");
  });
});

test("applyUpdate starts a detached process and returns immediately", async () => {
  await withScenario("update_available", async () => {
    const started = Date.now();
    const response = await applyUpdate(client, { mustCloseProcessNames: "App1,App2", closeTimeoutSeconds: 45 });
    assert.equal(response.isSuccess, true);
    assert.ok(Date.now() - started < 2000);
  });
});

test("self update flow", async () => {
  await withScenario("self_update", async () => {
    const check = await checkSelfUpdate(client);
    assert.equal(check.data.need_update, true);
    const updated = await selfUpdate(client);
    assert.equal(updated.isSuccess, true);
    assert.equal(updated.data.updated, true);
    assert.equal(updated.data.old_path, "aly-client.exe.old");
  });
});

test("rollback flow", async () => {
  await withScenario("rollback", async () => {
    const versions = await listRollbackVersions(client);
    assert.deepEqual(versions.data.versions, ["1.0.0", "0.9.0"]);
    const rolled = await rollback(client, "1.0.0");
    assert.equal(rolled.isSuccess, true);
    assert.equal(rolled.data.version, "1.0.0");
  });
});

test("checkDiff and clean", async () => {
  await withScenario("check_diff", async () => {
    const diff = await checkDiff(client);
    assert.equal(diff.isSuccess, true);
    assert.equal(diff.data.files[0].path, "app.exe");
  });
  await withScenario("clean_all", async () => {
    const cleaned = await clean(client, { all: true });
    assert.equal(cleaned.isSuccess, true);
    assert.equal(cleaned.data.mode, "all");
  });
});

test("missing executable becomes a failed response", async () => {
  const check = await checkUpdate("/definitely/not/a/real/aly-client");
  assert.equal(check.isSuccess, false);
  assert.ok(check.errorMsg.length > 0);

  const download = await downloadUpdate("/definitely/not/a/real/aly-client");
  assert.equal(download.isSuccess, false);
});

test("defaultClientPath points at ../UpdateFolder", () => {
  assert.match(defaultClientPath(), /UpdateFolder/);
});