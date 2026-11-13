use crate::api::{
    Api, CancelToken, CloseOptions, DownloadOptions, DEFAULT_DOWNLOAD_STALL_TIMEOUT,
};
use crate::Response;

use std::panic::{catch_unwind, AssertUnwindSafe};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

/// 更新生命周期阶段。
pub struct Status;

impl Status {
    pub const NONE: &'static str = "none";
    pub const DISCOVERED_UPDATE: &'static str = "discoveredUpdate";
    pub const DOWNLOADING_UPDATE: &'static str = "downloadingUpdate";
    pub const DOWNLOADED_UPDATE: &'static str = "downloadedUpdate";
    pub const APPLY_UPDATE: &'static str = "applyUpdate";
}

type StatusFn = Box<dyn Fn(&str, &str) + Send + 'static>;
type ErrorFn = Box<dyn Fn(&str) + Send + 'static>;
type ConfirmFn = Box<dyn Fn(&str, &CancelToken) + Send + 'static>;

/// aly-client 的托管生命周期：自更新 → check → download → apply。
///
/// 确认回调在循环线程内同步执行，签名 `(version, cancel_token)`；返回即确认。
/// 阻塞时应轮询 token，保证 `stop` 能及时退出。
pub struct UpdateClient {
    api: Api,
    idle_poll: Duration,
    must_close_process_names: Option<String>,
    close_timeout_seconds: u32,
    status: Arc<Mutex<String>>,
    error: Arc<Mutex<String>>,
    running: Arc<AtomicBool>,
    cancelled: Arc<AtomicBool>,
    handle: Option<JoinHandle<()>>,
    on_status: Option<StatusFn>,
    on_error: Option<ErrorFn>,
    confirm_download: Option<ConfirmFn>,
    confirm_apply: Option<ConfirmFn>,
}

impl UpdateClient {
    pub fn new(api: Api) -> Self {
        Self {
            api,
            idle_poll: Duration::from_secs(300),
            must_close_process_names: None,
            close_timeout_seconds: 30,
            status: Arc::new(Mutex::new(Status::NONE.to_string())),
            error: Arc::new(Mutex::new(String::new())),
            running: Arc::new(AtomicBool::new(false)),
            cancelled: Arc::new(AtomicBool::new(false)),
            handle: None,
            on_status: None,
            on_error: None,
            confirm_download: None,
            confirm_apply: None,
        }
    }

    pub fn with_idle_poll(mut self, interval: Duration) -> Self {
        self.idle_poll = interval;
        self
    }

    pub fn with_must_close_process_names(mut self, names: impl Into<String>) -> Self {
        self.must_close_process_names = Some(names.into());
        self
    }

    pub fn with_close_timeout_seconds(mut self, seconds: u32) -> Self {
        self.close_timeout_seconds = seconds;
        self
    }

    pub fn on_status<F>(mut self, callback: F) -> Self
    where
        F: Fn(&str, &str) + Send + 'static,
    {
        self.on_status = Some(Box::new(callback));
        self
    }

    pub fn on_error<F>(mut self, callback: F) -> Self
    where
        F: Fn(&str) + Send + 'static,
    {
        self.on_error = Some(Box::new(callback));
        self
    }

    pub fn confirm_download<F>(mut self, callback: F) -> Self
    where
        F: Fn(&str, &CancelToken) + Send + 'static,
    {
        self.confirm_download = Some(Box::new(callback));
        self
    }

    pub fn confirm_apply<F>(mut self, callback: F) -> Self
    where
        F: Fn(&str, &CancelToken) + Send + 'static,
    {
        self.confirm_apply = Some(Box::new(callback));
        self
    }

    pub fn status(&self) -> String {
        lock(&self.status).clone()
    }

    pub fn last_error(&self) -> String {
        lock(&self.error).clone()
    }

    pub fn is_running(&self) -> bool {
        self.handle
            .as_ref()
            .map(|handle| !handle.is_finished())
            .unwrap_or(false)
    }

    /// 启动后台循环（立即返回）。一个实例只启动一次。
    pub fn start(&mut self) {
        if self.is_running() {
            return;
        }
        self.running.store(true, Ordering::SeqCst);
        self.cancelled.store(false, Ordering::SeqCst);
        let api = self.api.clone();
        let idle_poll = self.idle_poll;
        let close_timeout_seconds = self.close_timeout_seconds;
        let must_close_process_names = self.must_close_process_names.clone();
        let status = Arc::clone(&self.status);
        let error = Arc::clone(&self.error);
        let running = Arc::clone(&self.running);
        let cancelled = Arc::clone(&self.cancelled);
        let on_status = self.on_status.take();
        let on_error = self.on_error.take();
        let confirm_download = self.confirm_download.take();
        let confirm_apply = self.confirm_apply.take();

        self.handle = Some(thread::spawn(move || {
            run_loop(
                api,
                idle_poll,
                close_timeout_seconds,
                must_close_process_names,
                status,
                error,
                running,
                cancelled,
                on_status,
                on_error,
                confirm_download,
                confirm_apply,
            );
        }));
    }

    /// 停止轮询并等待循环退出；返回是否在超时内退出。
    pub fn stop(&mut self, timeout: Duration) -> bool {
        self.running.store(false, Ordering::SeqCst);
        self.cancelled.store(true, Ordering::SeqCst);
        let handle = match self.handle.take() {
            Some(handle) => handle,
            None => return true,
        };
        let deadline = Instant::now() + timeout;
        while !handle.is_finished() && Instant::now() < deadline {
            thread::sleep(Duration::from_millis(10));
        }
        if handle.is_finished() {
            let _ = handle.join();
            // 审查 Rust 轻微①：停止成功后清空回调，避免宿主在 stop 之后
            // 仍收到迟到的状态/错误/确认回调（UI 已关闭时这类回调会触碰失效状态）。
            self.on_status = None;
            self.on_error = None;
            self.confirm_download = None;
            self.confirm_apply = None;
            true
        } else {
            self.handle = Some(handle);
            false
        }
    }
}

#[allow(clippy::too_many_arguments)]
fn run_loop(
    api: Api,
    idle_poll: Duration,
    close_timeout_seconds: u32,
    must_close_process_names: Option<String>,
    status: Arc<Mutex<String>>,
    error: Arc<Mutex<String>>,
    running: Arc<AtomicBool>,
    cancelled: Arc<AtomicBool>,
    on_status: Option<StatusFn>,
    on_error: Option<ErrorFn>,
    confirm_download: Option<ConfirmFn>,
    confirm_apply: Option<ConfirmFn>,
) {
    let token = CancelToken::from_flag(cancelled);
    try_self_update(&api, &token, &error, &on_error);

    while !token.is_cancelled() {
        let check = api.check_update(Some(&token));
        if token.is_cancelled() {
            return;
        }
        if !check.is_success {
            report_error(&error, &on_error, &check.error_msg);
            if !sleep(&running, Duration::from_secs(5)) {
                return;
            }
            continue;
        }
        clear_error(&error, &on_error);

        let data = match check.data {
            Some(data) => data,
            None => {
                report_error(&error, &on_error, "check_update returned no data");
                if !sleep(&running, Duration::from_secs(5)) {
                    return;
                }
                continue;
            }
        };

        if !data.has_update {
            if lock(&status).as_str() != Status::NONE {
                set_status(&status, &on_status, Status::NONE, "Update status cleared");
            }
            if !sleep(&running, idle_delay(idle_poll)) {
                return;
            }
            continue;
        }

        let force = data.force_update.unwrap_or(false);
        let version = data.new_version.clone().unwrap_or_default();
        // 三态分派（与 C# UpdateClient 一致）：has_update && !need_download_update
        // 表示本地已下载待应用 / applying 崩溃恢复 / 回滚续跑，直接 apply，
        // 不下载、不询问（恢复场景自愈）。
        if data.need_download_update {
            let message = if version.is_empty() {
                "Found new version".to_string()
            } else {
                format!("Found new version {version}")
            };
            set_status(&status, &on_status, Status::DISCOVERED_UPDATE, &message);
            if !force {
                call_confirm(confirm_download.as_ref(), &version, &token, &error, &on_error);
                if token.is_cancelled() {
                    return;
                }
            }

            set_status(
                &status,
                &on_status,
                Status::DOWNLOADING_UPDATE,
                "Downloading update...",
            );
            let downloaded = {
                let mut on_progress = |info: crate::DownloadProgressInfo| {
                    set_status(
                        &status,
                        &on_status,
                        Status::DOWNLOADING_UPDATE,
                        &format!(
                            "Downloading {}... {}%",
                            info.file,
                            (info.ratio * 100.0) as i32
                        ),
                    );
                };
                api.download_update(DownloadOptions {
                    stall_timeout: DEFAULT_DOWNLOAD_STALL_TIMEOUT,
                    on_progress: Some(&mut on_progress),
                    cancel: Some(&token),
                })
            };
            if token.is_cancelled() {
                return;
            }
            if !downloaded.is_success {
                if downloaded.error_msg.contains("already at latest version") {
                    set_status(&status, &on_status, Status::NONE, "Update status cleared");
                    if !sleep(&running, idle_delay(idle_poll)) {
                        return;
                    }
                    continue;
                }
                report_error(&error, &on_error, &downloaded.error_msg);
                if !sleep(&running, Duration::from_secs(1)) {
                    return;
                }
                continue;
            }
        }

        set_status(
            &status,
            &on_status,
            Status::DOWNLOADED_UPDATE,
            "Update downloaded",
        );
        if !force {
            call_confirm(confirm_apply.as_ref(), &version, &token, &error, &on_error);
            if token.is_cancelled() {
                return;
            }
        }

        set_status(&status, &on_status, Status::APPLY_UPDATE, "Applying update...");
        let applied = api.apply_update(&CloseOptions {
            must_close_process_names: must_close_process_names.clone(),
            close_timeout_seconds,
        });
        if !applied.is_success {
            report_error(
                &error,
                &on_error,
                &format!("apply spawn failed: {}", applied.error_msg),
            );
        }
        if !sleep(&running, Duration::from_secs(30)) {
            return;
        }
    }
}

fn try_self_update(
    api: &Api,
    token: &CancelToken,
    error: &Arc<Mutex<String>>,
    on_error: &Option<ErrorFn>,
) {
    let check = api.check_self_update(Some(token));
    if token.is_cancelled() || !check.is_success {
        return;
    }
    let need_update = check.data.map(|data| data.need_update).unwrap_or(false);
    if !need_update {
        return;
    }
    let updated: Response<crate::SelfUpdateData> = api.self_update(Some(token));
    if !updated.is_success {
        report_error(error, on_error, &updated.error_msg);
    }
}

fn call_confirm(
    handler: Option<&ConfirmFn>,
    version: &str,
    token: &CancelToken,
    error: &Arc<Mutex<String>>,
    on_error: &Option<ErrorFn>,
) {
    let Some(handler) = handler else {
        return;
    };
    if catch_unwind(AssertUnwindSafe(|| handler(version, token))).is_err() {
        report_error(error, on_error, "confirm handler panicked");
    }
}

fn set_status(
    status: &Arc<Mutex<String>>,
    on_status: &Option<StatusFn>,
    next: &str,
    message: &str,
) {
    *lock(status) = next.to_string();
    if let Some(callback) = on_status {
        let _ = catch_unwind(AssertUnwindSafe(|| callback(next, message)));
    }
}

fn report_error(error: &Arc<Mutex<String>>, on_error: &Option<ErrorFn>, message: &str) {
    let next = message.to_string();
    {
        let mut current = lock(error);
        if *current == next {
            return;
        }
        *current = next.clone();
    }
    if let Some(callback) = on_error {
        let _ = catch_unwind(AssertUnwindSafe(|| callback(&next)));
    }
}

fn clear_error(error: &Arc<Mutex<String>>, on_error: &Option<ErrorFn>) {
    {
        let mut current = lock(error);
        if current.is_empty() {
            return;
        }
        *current = String::new();
    }
    if let Some(callback) = on_error {
        let _ = catch_unwind(AssertUnwindSafe(|| callback("")));
    }
}

/// 返回 true 表示循环应继续。
fn sleep(running: &AtomicBool, duration: Duration) -> bool {
    let deadline = Instant::now() + duration;
    while running.load(Ordering::SeqCst) && Instant::now() < deadline {
        let remaining = deadline.saturating_duration_since(Instant::now());
        thread::sleep(remaining.min(Duration::from_millis(100)));
    }
    running.load(Ordering::SeqCst)
}

/// 空闲轮询的实际间隔（带抖动）。
///
/// 审查 Rust 轻微②：旧实现抖动 = `当前纳秒 % 61`（0..=60s）。
/// 问题有二：① 抖动可达 +60s——interval=60s 时最长要 120s 才检查一次，
/// 与文档"每 N 秒检查一次"不符，静默期/更新响应被成倍拉长；
/// ② 每次 sleep 都不同，行为完全不可预测（测试也没法断言）。
/// 现在改为**按进程确定**的偏移：幅度 ≤ 10% 且 ≤ 30s，不同机器错峰、同一进程稳定。
fn idle_delay(interval: Duration) -> Duration {
    let seconds = interval.as_secs().max(60);
    let cap = (seconds / 10).clamp(1, 30) as i64;
    let hash = (std::process::id() as u64).wrapping_mul(2654435761);
    let delta = (hash % ((cap * 2 + 1) as u64)) as i64 - cap;
    let total = (seconds as i64 + delta).max(1) as u64;
    Duration::from_secs(total)
}

fn lock(value: &Arc<Mutex<String>>) -> std::sync::MutexGuard<'_, String> {
    value.lock().unwrap_or_else(|poisoned| poisoned.into_inner())
}