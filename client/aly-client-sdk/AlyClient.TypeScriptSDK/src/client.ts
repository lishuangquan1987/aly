import {
  applyUpdate,
  checkSelfUpdate,
  checkUpdate,
  defaultClientPath,
  downloadUpdate,
  selfUpdate,
} from "./api";

/** 更新生命周期阶段。 */
export type AlyClientStatus =
  | "none"
  | "discoveredUpdate"
  | "downloadingUpdate"
  | "downloadedUpdate"
  | "applyUpdate";

export interface AlyUpdateClientOptions {
  /** 更新器路径；默认 `<cwd>/../UpdateFolder/aly-client[.exe]`。 */
  clientPath?: string;
  /** 逗号分隔、不带 .exe 的进程名，传给 apply_update。 */
  mustCloseProcessNames?: string;
  /** 关闭进程超时（秒），默认 30。 */
  closeTimeoutSeconds?: number;
  /** 空闲轮询间隔（秒）：默认 300，最小 60，叠加 0–60s 随机抖动。 */
  idlePollIntervalSeconds?: number;
  /** 每次状态迁移触发；消息已包含下载文件名与百分比。 */
  onStatusChange?: (status: AlyClientStatus, message: string) => void;
  /** 错误出现/变化时触发；恢复时参数为 null。 */
  onError?: (message: string | null) => void;
  /** 非强制更新的下载确认；返回即确认。未提供时视为自动确认。 */
  confirmDownload?: (newVersion: string) => void | Promise<void>;
  /** 非强制更新的应用确认；返回即确认。 */
  confirmApply?: (newVersion: string) => void | Promise<void>;
  /** 子进程环境（测试用，默认继承 process.env）。 */
  env?: NodeJS.ProcessEnv;
}

/**
 * `aly-client` 的托管生命周期：自更新 → check → download → apply，
 * 带空闲轮询、确认回调与可取消的等待。
 */
export class AlyUpdateClient {
  readonly clientPath: string;

  private readonly options: AlyUpdateClientOptions;
  private readonly abortedPromise: Promise<void>;
  private resolveAborted!: () => void;

  private loopPromise: Promise<void> | null = null;
  private statusValue: AlyClientStatus = "none";
  private lastError: string | null = null;
  private disposed = false;

  constructor(options: AlyUpdateClientOptions = {}) {
    this.options = options;
    this.clientPath = options.clientPath ?? defaultClientPath();
    this.abortedPromise = new Promise<void>((resolve) => {
      this.resolveAborted = resolve;
    });
  }

  /** 当前阶段。 */
  get status(): AlyClientStatus {
    return this.statusValue;
  }

  /** 最近一次错误；null 表示已恢复。 */
  get error(): string | null {
    return this.lastError;
  }

  /** 后台循环是否在运行。 */
  get isRunning(): boolean {
    return this.loopPromise !== null;
  }

  /** 启动后台循环（立即返回）。重复调用无副作用。 */
  start(): void {
    if (this.disposed || this.loopPromise) return;
    this.loopPromise = this.runLoop()
      .catch((err) => {
        this.reportError(`update exception: ${message(err)}`);
      })
      .finally(() => {
        this.loopPromise = null;
      });
  }

  /**
   * 停止轮询并等待循环退出（最长约 1 秒）。
   * 不结束由 apply_update 拉起的进程，也不删除任何文件。
   */
  async dispose(): Promise<void> {
    if (!this.disposed) {
      this.disposed = true;
      this.resolveAborted();
    }
    if (this.loopPromise) {
      await this.loopPromise.catch(() => {
        /* 循环内部已处理 */
      });
    }
  }

  private async runLoop(): Promise<void> {
    await this.trySelfUpdate();
    while (!this.disposed) {
      const check = await checkUpdate(this.clientPath, { env: this.options.env });
      if (this.disposed) return;

      if (!check.isSuccess) {
        this.reportError(check.errorMsg || "check_update failed");
        await this.sleep(5000);
        continue;
      }
      if (this.lastError !== null) this.reportError(null);

      const data = check.data;
      if (!data) {
        this.reportError("check_update returned no data");
        await this.sleep(5000);
        continue;
      }
      if (!data.has_update) {
        if (this.statusValue !== "none") {
          this.setStatus("none", "Update status cleared");
        }
        await this.sleep(this.idleDelayMs());
        continue;
      }

      const version = data.new_version ?? "";
      // 三态分派（与 C# AlyUpdateClient 一致）：has_update && !need_download_update
      // 表示本地已下载待应用 / applying 崩溃恢复 / 回滚续跑，直接 apply，
      // 不下载、不询问（恢复场景自愈）。
      if (data.need_download_update) {
        this.setStatus("discoveredUpdate", version ? `Found new version ${version}` : "Found new version");
        if (!data.force_update) {
          await this.waitConfirm(this.options.confirmDownload, version);
          if (this.disposed) return;
        }

        this.setStatus("downloadingUpdate", "Downloading update...");
        const downloaded = await downloadUpdate(
          this.clientPath,
          (progress) => {
            this.setStatus(
              "downloadingUpdate",
              `Downloading ${progress.file}... ${Math.round(progress.ratio * 100)}%`,
            );
          },
          { env: this.options.env },
        );
        if (this.disposed) return;

        if (!downloaded.isSuccess) {
          if ((downloaded.errorMsg ?? "").includes("already at latest version")) {
            this.setStatus("none", "Update status cleared");
            await this.sleep(this.idleDelayMs());
            continue;
          }
          this.reportError(downloaded.errorMsg || "download failed");
          await this.sleep(1000);
          continue;
        }
      }

      this.setStatus("downloadedUpdate", "Update downloaded");
      if (!data.force_update) {
        await this.waitConfirm(this.options.confirmApply, version);
        if (this.disposed) return;
      }

      this.setStatus("applyUpdate", "Applying update...");
      const applied = await applyUpdate(this.clientPath, {
        env: this.options.env,
        mustCloseProcessNames: this.options.mustCloseProcessNames,
        closeTimeoutSeconds: this.options.closeTimeoutSeconds,
      });
      if (!applied.isSuccess) {
        this.reportError(`apply spawn failed: ${applied.errorMsg}`);
      }
      // 更新器持有锁并会关闭宿主；30s 内不要重复拉起。
      await this.sleep(30000);
    }
  }

  private async trySelfUpdate(): Promise<void> {
    try {
      const check = await checkSelfUpdate(this.clientPath, { env: this.options.env });
      if (!check.isSuccess || !check.data.need_update) return;
      const updated = await selfUpdate(this.clientPath, { env: this.options.env });
      if (!updated.isSuccess) {
        this.reportError(updated.errorMsg || "self_update failed");
      }
    } catch (err) {
      this.reportError(`self update exception: ${message(err)}`);
    }
  }

  private async waitConfirm(
    callback: ((newVersion: string) => void | Promise<void>) | undefined,
    version: string,
  ): Promise<void> {
    if (!callback) return; // 没提供确认回调视为已确认（与 C# SDK 一致）
    try {
      await Promise.race([Promise.resolve(callback(version)), this.abortedPromise]);
    } catch (err) {
      this.reportError(`confirm handler error: ${message(err)}`);
    }
  }

  private idleDelayMs(): number {
    let seconds = this.options.idlePollIntervalSeconds ?? 300;
    if (!(seconds > 0)) seconds = 300;
    if (seconds < 60) seconds = 60;
    return (seconds + Math.random() * 60) * 1000;
  }

  private setStatus(status: AlyClientStatus, message: string): void {
    this.statusValue = status;
    try {
      this.options.onStatusChange?.(status, message);
    } catch {
      /* 回调异常不得打断循环 */
    }
  }

  private reportError(errorMsg: string | null): void {
    if (this.lastError === errorMsg) return;
    this.lastError = errorMsg;
    try {
      this.options.onError?.(errorMsg);
    } catch {
      /* 回调异常不得打断循环 */
    }
  }

  private async sleep(ms: number): Promise<void> {
    const deadline = Date.now() + ms;
    while (!this.disposed && Date.now() < deadline) {
      const chunk = Math.min(1000, deadline - Date.now());
      await Promise.race([delay(chunk), this.abortedPromise]);
    }
  }
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function message(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}