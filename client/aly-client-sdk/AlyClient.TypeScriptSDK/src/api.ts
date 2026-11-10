import { spawn } from "node:child_process";
import * as path from "node:path";
import * as readline from "node:readline";
import type {
  CheckDiffData,
  CheckSelfUpdateData,
  CheckUpdateData,
  CleanData,
  DownloadProgress,
  RollbackData,
  RollbackListData,
  SelfUpdateData,
} from "./models";
import type { AlyResponse } from "./protocol";
import { ng, ok } from "./protocol";

/** 短命令默认超时（秒）。 */
export const DEFAULT_SHORT_TIMEOUT_SECONDS = 30;
/** 下载停流看门狗默认值（秒）。 */
export const DEFAULT_DOWNLOAD_STALL_TIMEOUT_SECONDS = 1800;
/** 回滚默认超时（秒）。 */
export const ROLLBACK_TIMEOUT_SECONDS = 600;

/** 子进程环境（默认继承 process.env；测试用）。 */
export interface ApiOptions {
  env?: NodeJS.ProcessEnv;
}

export interface ShortCommandOptions extends ApiOptions {
  /** 覆盖短命令超时（毫秒）。 */
  timeoutMs?: number;
}

export interface DownloadOptions extends ApiOptions {
  /** 覆盖停流看门狗（秒）。 */
  stallTimeoutSeconds?: number;
}

export interface CloseProcessOptions extends ApiOptions {
  /** 逗号分隔、不带 .exe 的进程名。 */
  mustCloseProcessNames?: string;
  /** 关闭进程超时（秒），默认 30；等于 30 时不传参。 */
  closeTimeoutSeconds?: number;
}

export interface CleanOptions extends ShortCommandOptions {
  /** 保留最近 N 个版本快照。 */
  keep?: number;
  /** 激进清理，等效 keep=1 但输出 mode=all。 */
  all?: boolean;
}

/** 下载进度（文件计数粒度；字节字段可选）。 */
export interface DownloadProgressInfo {
  file: string;
  ratio: number;
  status: DownloadProgress["status"];
  index: number;
  total: number;
  downloadedBytes?: number;
  totalBytes?: number;
  speedBps?: number;
}

export type ProgressCallback = (info: DownloadProgressInfo) => void;

/** 默认更新器路径：<cwd>/../UpdateFolder/aly-client[.exe]。 */
export function defaultClientPath(): string {
  const exe = process.platform === "win32" ? "aly-client.exe" : "aly-client";
  return path.resolve(process.cwd(), "..", "UpdateFolder", exe);
}

interface RunResult {
  code: number | null;
  stdout: string;
  stderr: string;
  timedOut: boolean;
  spawnError?: string;
}

function runOnce(
  clientPath: string,
  args: string[],
  timeoutMs: number,
  env?: NodeJS.ProcessEnv,
): Promise<RunResult> {
  return new Promise((resolve) => {
    let settled = false;
    let timedOut = false;
    let stdout = "";
    let stderr = "";
    let timer: NodeJS.Timeout | undefined;

    const finish = (result: RunResult): void => {
      if (settled) return;
      settled = true;
      if (timer) clearTimeout(timer);
      resolve(result);
    };

    let child;
    try {
      child = spawn(clientPath, args, {
        cwd: path.dirname(clientPath),
        windowsHide: true,
        stdio: ["ignore", "pipe", "pipe"],
        env: env ?? process.env,
      });
    } catch (err) {
      finish({ code: null, stdout: "", stderr: "", timedOut: false, spawnError: message(err) });
      return;
    }

    child.stdout?.setEncoding("utf8");
    child.stderr?.setEncoding("utf8");
    child.stdout?.on("data", (chunk: string) => {
      stdout += chunk;
    });
    child.stderr?.on("data", (chunk: string) => {
      stderr += chunk;
    });
    child.once("error", (err) => {
      finish({ code: null, stdout, stderr, timedOut, spawnError: message(err) });
    });
    child.once("close", (code) => {
      finish({ code, stdout, stderr, timedOut });
    });

    if (timeoutMs > 0) {
      timer = setTimeout(() => {
        timedOut = true;
        try {
          child.kill();
        } catch {
          /* already gone */
        }
      }, timeoutMs);
    }
  });
}

function message(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

function describeFailure(result: RunResult, timeoutMs: number): string {
  if (result.spawnError) return result.spawnError;
  if (result.timedOut) {
    return `aly-client timed out after ${Math.round(timeoutMs / 1000)}s and was killed`;
  }
  const errText = result.stderr.trim();
  if (result.code !== 0) {
    return errText || `aly-client exited with code ${result.code} (may require admin)`;
  }
  return "aly-client returned no output (may require admin)";
}

function parse<T>(result: RunResult, timeoutMs: number): AlyResponse<T> {
  const text = result.stdout.trim();
  if (!text) return ng(describeFailure(result, timeoutMs));
  try {
    // 退出码非 0 但 stdout 有 JSON 时信任 JSON（host-contract §2）。
    return JSON.parse(text) as AlyResponse<T>;
  } catch (err) {
    return ng(message(err));
  }
}

async function runShort<T>(
  command: string,
  clientPath: string,
  options: ShortCommandOptions,
): Promise<AlyResponse<T>> {
  const timeoutMs = options.timeoutMs ?? DEFAULT_SHORT_TIMEOUT_SECONDS * 1000;
  const result = await runOnce(clientPath, [command], timeoutMs, options.env);
  return parse<T>(result, timeoutMs);
}

/** 检查应用是否有新版本。服务器不可达时 has_update=false，不报错。 */
export function checkUpdate(
  clientPath: string = defaultClientPath(),
  options: ShortCommandOptions = {},
): Promise<AlyResponse<CheckUpdateData>> {
  return runShort("check_update", clientPath, options);
}

/** 检查更新器自身副本是否需要更新。 */
export function checkSelfUpdate(
  clientPath: string = defaultClientPath(),
  options: ShortCommandOptions = {},
): Promise<AlyResponse<CheckSelfUpdateData>> {
  return runShort("check_self_update", clientPath, options);
}

/** 执行更新器自更新。 */
export function selfUpdate(
  clientPath: string = defaultClientPath(),
  options: ShortCommandOptions = {},
): Promise<AlyResponse<SelfUpdateData>> {
  return runShort("self_update", clientPath, options);
}

/** 列出可回滚的历史版本。 */
export function listRollbackVersions(
  clientPath: string = defaultClientPath(),
  options: ShortCommandOptions = {},
): Promise<AlyResponse<RollbackListData>> {
  return runShort("list_rollback_versions", clientPath, options);
}

/** 列出需要下载的差异文件（调试/可视化）。 */
export function checkDiff(
  clientPath: string = defaultClientPath(),
  options: ShortCommandOptions = {},
): Promise<AlyResponse<CheckDiffData>> {
  return runShort("check_diff", clientPath, options);
}

/** 清理历史快照与临时文件。 */
export function clean(
  clientPath: string = defaultClientPath(),
  options: CleanOptions = {},
): Promise<AlyResponse<CleanData>> {
  const args = ["clean"];
  if (options.all) {
    args.push("--all");
  } else if (options.keep !== undefined) {
    args.push("--keep", String(options.keep));
  }
  const timeoutMs = options.timeoutMs ?? DEFAULT_SHORT_TIMEOUT_SECONDS * 1000;
  return runOnce(clientPath, args, timeoutMs, options.env).then((r) => parse<CleanData>(r, timeoutMs));
}

function appendCloseArgs(args: string[], options: CloseProcessOptions): void {
  const names = options.mustCloseProcessNames?.trim();
  if (names) args.push("--must-close-process-name", names);
  const timeout = options.closeTimeoutSeconds;
  if (timeout !== undefined && timeout !== 30) args.push("--close-timeout", String(timeout));
}

/** 回滚到指定版本（600 秒超时）。 */
export async function rollback(
  clientPath: string = defaultClientPath(),
  version: string,
  options: CloseProcessOptions = {},
): Promise<AlyResponse<RollbackData>> {
  const args = ["rollback", "--version", version];
  appendCloseArgs(args, options);
  const timeoutMs = ROLLBACK_TIMEOUT_SECONDS * 1000;
  const result = await runOnce(clientPath, args, timeoutMs, options.env);
  return parse<RollbackData>(result, timeoutMs);
}

/**
 * 下载差异文件。没有总时长上限，只有停流看门狗；
 * 进度回调按文件计数，只统计 DONE 行。
 */
export function downloadUpdate(
  clientPath: string = defaultClientPath(),
  onProgress?: ProgressCallback,
  options: DownloadOptions = {},
): Promise<AlyResponse<null>> {
  const stallMs = (options.stallTimeoutSeconds ?? DEFAULT_DOWNLOAD_STALL_TIMEOUT_SECONDS) * 1000;

  return new Promise((resolve) => {
    let settled = false;
    let stalled = false;
    let timer: NodeJS.Timeout | undefined;
    let child;

    const finish = (response: AlyResponse<null>): void => {
      if (settled) return;
      settled = true;
      if (timer) clearTimeout(timer);
      resolve(response);
    };

    try {
      child = spawn(clientPath, ["download_update"], {
        cwd: path.dirname(clientPath),
        windowsHide: true,
        stdio: ["ignore", "pipe", "pipe"],
        env: options.env ?? process.env,
      });
    } catch (err) {
      finish(ng(message(err)));
      return;
    }

    child.stdout?.setEncoding("utf8");
    child.stderr?.setEncoding("utf8");
    let stderr = "";
    child.stderr?.on("data", (chunk: string) => {
      stderr += chunk;
    });

    if (!child.stdout) {
      finish(ng("failed to start aly-client (no stdout pipe)"));
      return;
    }

    const resetWatchdog = (): void => {
      if (timer) clearTimeout(timer);
      timer = setTimeout(() => {
        stalled = true;
        try {
          child.kill();
        } catch {
          /* already gone */
        }
      }, stallMs);
    };
    resetWatchdog();

    const rl = readline.createInterface({ input: child.stdout });
    rl.on("line", (line) => {
      resetWatchdog();
      const trimmed = line.trim();
      if (!trimmed) return;

      let env: AlyResponse<DownloadProgress | null>;
      try {
        env = JSON.parse(trimmed) as AlyResponse<DownloadProgress | null>;
      } catch (err) {
        finish(ng(`invalid progress line: ${message(err)}`));
        try {
          child.kill();
        } catch {
          /* already gone */
        }
        return;
      }

      if (!env.isSuccess) {
        // 与 C# SDK 一致：任一行失败即返回该行 errorMsg（host-contract §6）。
        finish(ng(env.errorMsg || "download failed"));
        try {
          child.kill();
        } catch {
          /* already gone */
        }
        return;
      }
      if (env.data === null) {
        finish(ok(null));
        return;
      }
      const data = env.data;
      if (data.status === "DONE" && data.total > 0) {
        try {
          onProgress?.({
            file: data.file,
            ratio: data.index / data.total,
            status: data.status,
            index: data.index,
            total: data.total,
            downloadedBytes: data.downloaded_bytes,
            totalBytes: data.total_bytes,
            speedBps: data.speed_bps,
          });
        } catch {
          /* 进度回调异常不得打断下载 */
        }
      }
    });

    child.once("error", (err) => {
      finish(ng(message(err)));
    });
    child.once("close", () => {
      if (settled) return;
      if (stalled) {
        finish(ng(`download stalled: no output for ${Math.round(stallMs / 1000)}s, process killed`));
      } else {
        finish(ng(stderr.trim() || "download interrupted (aly-client exited before finish line)"));
      }
    });
  });
}

/**
 * 应用已下载的更新：拉起独立进程后立即返回，不等待。
 * 更新器会关闭宿主并重启主程序，调用方不要在此之后做清理工作。
 */
export function applyUpdate(
  clientPath: string = defaultClientPath(),
  options: CloseProcessOptions = {},
): Promise<AlyResponse<null>> {
  const args = ["apply_update"];
  appendCloseArgs(args, options);

  return new Promise((resolve) => {
    let child;
    try {
      child = spawn(clientPath, args, {
        cwd: path.dirname(clientPath),
        detached: true,
        windowsHide: true,
        stdio: "ignore",
        env: options.env ?? process.env,
      });
    } catch (err) {
      resolve(ng(message(err)));
      return;
    }
    child.once("error", (err) => {
      resolve(ng(message(err)));
    });
    child.once("spawn", () => {
      child.unref();
      resolve(ok(null));
    });
  });
}