const { execFileSync } = require("node:child_process");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");

const repoRoot = path.resolve(__dirname, "../../../..");
const fakeDir = path.join(repoRoot, "tools", "fake-aly-client");

let builtFake = null;

/** 构建仓库里的假客户端，返回可执行文件绝对路径。 */
function fakeClient() {
  if (builtFake) return builtFake;
  const name = process.platform === "win32" ? "aly-client.exe" : "aly-client";
  const out = path.join(os.tmpdir(), `aly-fake-${process.pid}`, name);
  fs.mkdirSync(path.dirname(out), { recursive: true });
  execFileSync("go", ["build", "-o", out, "."], { cwd: fakeDir, stdio: "pipe" });
  builtFake = out;
  return out;
}

/** 仓库内 fixtures 场景的绝对路径。 */
function scenario(name) {
  return path.join(repoRoot, "spec", "fixtures", "scenarios", `${name}.json`);
}

/**
 * 用 ALY_FAKE_SCENARIO 指定场景运行 fn，结束后恢复原值。
 * SDK 拉起子进程时继承 process.env。
 */
async function withScenario(nameOrPath, fn) {
  const previous = process.env.ALY_FAKE_SCENARIO;
  process.env.ALY_FAKE_SCENARIO = nameOrPath.includes("/") || nameOrPath.includes("\\")
    ? nameOrPath
    : scenario(nameOrPath);
  try {
    return await fn();
  } finally {
    if (previous === undefined) delete process.env.ALY_FAKE_SCENARIO;
    else process.env.ALY_FAKE_SCENARIO = previous;
  }
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function waitFor(predicate, timeoutMs = 5000, intervalMs = 20) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (predicate()) return;
    await sleep(intervalMs);
  }
  throw new Error(`waitFor timeout after ${timeoutMs}ms`);
}

/** 写一个临时场景文件并返回绝对路径。 */
function tempScenario(content) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "aly-scenario-"));
  const file = path.join(dir, "scenario.json");
  fs.writeFileSync(file, JSON.stringify(content), "utf8");
  return file;
}

module.exports = { repoRoot, fakeClient, scenario, withScenario, sleep, waitFor, tempScenario };