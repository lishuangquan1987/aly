const test = require("node:test");
const assert = require("node:assert/strict");
const { AlyUpdateClient } = require("../dist");
const { fakeClient, withScenario, sleep, waitFor } = require("./helpers");

const clientPath = fakeClient();

test("managed loop: force update runs check -> download -> apply", async () => {
  await withScenario("force_update", async () => {
    const statuses = [];
    const errors = [];
    const client = new AlyUpdateClient({
      clientPath,
      onStatusChange: (status) => statuses.push(status),
      onError: (message) => errors.push(message),
    });

    client.start();
    await waitFor(() => statuses.includes("applyUpdate"), 8000);
    await client.dispose();

    assert.equal(statuses[0], "discoveredUpdate");
    assert.ok(statuses.includes("downloadingUpdate"));
    assert.ok(statuses.includes("downloadedUpdate"));
    assert.equal(statuses[statuses.length - 1], "applyUpdate");
    assert.deepEqual(errors, []);
    assert.equal(client.isRunning, false);
  });
});

test("managed loop: non-force update waits for confirmations", async () => {
  await withScenario("update_available", async () => {
    const downloads = [];
    const applies = [];
    const statuses = [];
    const client = new AlyUpdateClient({
      clientPath,
      confirmDownload: (version) => {
        downloads.push(version);
      },
      confirmApply: (version) => {
        applies.push(version);
      },
      onStatusChange: (status) => statuses.push(status),
    });

    client.start();
    await waitFor(() => statuses.includes("applyUpdate"), 8000);
    await client.dispose();

    assert.deepEqual(downloads, ["1.0.1"]);
    assert.deepEqual(applies, ["1.0.1"]);
  });
});

test("managed loop: dispose interrupts a pending confirmation", async () => {
  await withScenario("update_available", async () => {
    const client = new AlyUpdateClient({
      clientPath,
      confirmDownload: () => new Promise(() => {
        /* 永不返回，模拟用户迟迟不确认 */
      }),
    });

    client.start();
    await waitFor(() => client.status === "discoveredUpdate", 5000);
    const started = Date.now();
    await client.dispose();
    assert.ok(Date.now() - started < 2000, "dispose must not wait for the confirmation");
  });
});

test("managed loop: offline stays idle and does not ask for download", async () => {
  await withScenario("offline", async () => {
    let asked = false;
    const statuses = [];
    const client = new AlyUpdateClient({
      clientPath,
      confirmDownload: () => {
        asked = true;
      },
      onStatusChange: (status) => statuses.push(status),
    });

    client.start();
    await sleep(400);
    await client.dispose();

    assert.equal(asked, false);
    assert.equal(client.status, "none");
    assert.equal(statuses.includes("discoveredUpdate"), false);
  });
});

test("managed loop: pending apply (=true, need_download=false) applies without downloading", async () => {
  await withScenario("pending_apply", async () => {
    const statuses = [];
    const downloads = [];
    const applies = [];
    const errors = [];
    const client = new AlyUpdateClient({
      clientPath,
      confirmDownload: (version) => {
        downloads.push(version);
      },
      confirmApply: (version) => {
        applies.push(version);
      },
      onStatusChange: (status) => statuses.push(status),
      onError: (message) => errors.push(message),
    });

    client.start();
    await waitFor(() => statuses.includes("applyUpdate"), 8000);
    await client.dispose();

    assert.deepEqual(downloads, [], "pending apply must not ask for download");
    assert.equal(statuses.includes("discoveredUpdate"), false);
    assert.equal(statuses.includes("downloadingUpdate"), false);
    assert.deepEqual(applies, ["1.0.1"], "must ask for apply confirmation");
    assert.equal(statuses[0], "downloadedUpdate");
    assert.equal(statuses[statuses.length - 1], "applyUpdate");
    assert.deepEqual(errors, []);
  });
});

test("managed loop: dispose is idempotent and state stays readable", async () => {
  const client = new AlyUpdateClient({ clientPath });
  client.start();
  await sleep(50);
  await client.dispose();
  await client.dispose();
  assert.equal(client.isRunning, false);
  assert.equal(typeof client.status, "string");
});