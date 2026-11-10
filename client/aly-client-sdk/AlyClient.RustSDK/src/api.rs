use crate::models_gen::*;
use crate::Response;

use serde::de::DeserializeOwned;
use serde::Deserialize;
use std::io::{BufRead, BufReader, Read};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::mpsc::{self, RecvTimeoutError};
use std::sync::Arc;
use std::thread;
use std::time::{Duration, Instant};

pub const DEFAULT_SHORT_TIMEOUT: Duration = Duration::from_secs(30);
pub const ROLLBACK_TIMEOUT: Duration = Duration::from_secs(600);
pub const DEFAULT_DOWNLOAD_STALL_TIMEOUT: Duration = Duration::from_secs(1800);

/// 取消令牌；生命周期停止时 cancel()。
#[derive(Clone, Debug, Default)]
pub struct CancelToken {
    flag: Arc<AtomicBool>,
}

impl CancelToken {
    pub fn new() -> Self {
        Self {
            flag: Arc::new(AtomicBool::new(false)),
        }
    }

    pub(crate) fn from_flag(flag: Arc<AtomicBool>) -> Self {
        Self { flag }
    }

    /// 永不取消（测试/一次性调用）。
    pub fn never() -> Self {
        Self::new()
    }

    pub fn cancel(&self) {
        self.flag.store(true, Ordering::SeqCst);
    }

    pub fn is_cancelled(&self) -> bool {
        self.flag.load(Ordering::SeqCst)
    }
}

/// apply_update / rollback 的进程关闭参数。
#[derive(Debug, Clone, Default)]
pub struct CloseOptions {
    pub must_close_process_names: Option<String>,
    /// 0 或 30 时不传 `--close-timeout`。
    pub close_timeout_seconds: u32,
}

impl CloseOptions {
    pub fn with_names(names: impl Into<String>) -> Self {
        Self {
            must_close_process_names: Some(names.into()),
            close_timeout_seconds: 30,
        }
    }
}

/// clean 命令参数。
#[derive(Debug, Clone, Default)]
pub struct CleanOptions {
    pub keep: Option<u32>,
    pub all: bool,
}

/// 下载进度（文件计数粒度；字节字段可选）。
#[derive(Debug, Clone, PartialEq)]
pub struct DownloadProgressInfo {
    pub file: String,
    pub ratio: f64,
    pub status: String,
    pub index: i64,
    pub total: i64,
    pub downloaded_bytes: i64,
    pub total_bytes: i64,
    pub speed_bps: i64,
}

pub struct DownloadOptions<'a> {
    pub stall_timeout: Duration,
    pub on_progress: Option<&'a mut dyn FnMut(DownloadProgressInfo)>,
    pub cancel: Option<&'a CancelToken>,
}

impl<'a> DownloadOptions<'a> {
    pub fn new() -> Self {
        Self {
            stall_timeout: DEFAULT_DOWNLOAD_STALL_TIMEOUT,
            on_progress: None,
            cancel: None,
        }
    }
}

impl<'a> Default for DownloadOptions<'a> {
    fn default() -> Self {
        Self::new()
    }
}

/// 封装一个 aly-client 可执行文件。
#[derive(Clone, Debug)]
pub struct Api {
    client_path: PathBuf,
    env: Vec<(String, String)>,
}

impl Api {
    pub fn new(path: impl Into<PathBuf>) -> Self {
        let mut path = path.into();
        if !path.is_absolute() {
            if let Ok(cwd) = std::env::current_dir() {
                path = cwd.join(path);
            }
        }
        Self {
            client_path: path,
            env: Vec::new(),
        }
    }

    pub fn with_env(mut self, key: impl Into<String>, value: impl Into<String>) -> Self {
        self.env.push((key.into(), value.into()));
        self
    }

    pub fn client_path(&self) -> &Path {
        &self.client_path
    }

    /// 默认更新器路径：<可执行文件目录>/../UpdateFolder/aly-client[.exe]。
    pub fn default_path() -> PathBuf {
        let name = if cfg!(windows) {
            "aly-client.exe"
        } else {
            "aly-client"
        };
        let base = std::env::current_exe()
            .ok()
            .and_then(|p| p.parent().map(Path::to_path_buf))
            .or_else(|| std::env::current_dir().ok())
            .unwrap_or_default();
        base.join("..").join("UpdateFolder").join(name)
    }

    fn command(&self, args: &[String]) -> Command {
        let mut cmd = Command::new(&self.client_path);
        cmd.args(args);
        if let Some(parent) = self.client_path.parent() {
            cmd.current_dir(parent);
        }
        cmd.stdin(Stdio::null())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped());
        for (key, value) in &self.env {
            cmd.env(key, value);
        }
        hide_window(&mut cmd);
        cmd
    }

    pub fn check_update(&self, cancel: Option<&CancelToken>) -> Response<CheckUpdateData> {
        self.run_simple("check_update", DEFAULT_SHORT_TIMEOUT, cancel)
    }

    pub fn check_self_update(
        &self,
        cancel: Option<&CancelToken>,
    ) -> Response<CheckSelfUpdateData> {
        self.run_simple("check_self_update", DEFAULT_SHORT_TIMEOUT, cancel)
    }

    pub fn self_update(&self, cancel: Option<&CancelToken>) -> Response<SelfUpdateData> {
        self.run_simple("self_update", DEFAULT_SHORT_TIMEOUT, cancel)
    }

    pub fn list_rollback_versions(&self) -> Response<RollbackListData> {
        self.run_simple("list_rollback_versions", DEFAULT_SHORT_TIMEOUT, None)
    }

    pub fn check_diff(&self) -> Response<CheckDiffData> {
        self.run_simple("check_diff", DEFAULT_SHORT_TIMEOUT, None)
    }

    pub fn clean(&self, options: CleanOptions) -> Response<CleanData> {
        let mut args = vec!["clean".to_string()];
        if options.all {
            args.push("--all".to_string());
        } else if let Some(keep) = options.keep {
            args.push("--keep".to_string());
            args.push(keep.to_string());
        }
        parse(
            self.run_collect(&args, DEFAULT_SHORT_TIMEOUT, None),
            DEFAULT_SHORT_TIMEOUT,
        )
    }

    pub fn rollback(&self, version: &str, options: &CloseOptions) -> Response<RollbackData> {
        let mut args = vec![
            "rollback".to_string(),
            "--version".to_string(),
            version.to_string(),
        ];
        append_close_args(&mut args, options);
        parse(
            self.run_collect(&args, ROLLBACK_TIMEOUT, None),
            ROLLBACK_TIMEOUT,
        )
    }

    /// 拉起独立更新进程后立即返回，不等待。
    pub fn apply_update(&self, options: &CloseOptions) -> Response<()> {
        let mut args = vec!["apply_update".to_string()];
        append_close_args(&mut args, options);
        let mut cmd = self.command(&args);
        cmd.stdout(Stdio::null()).stderr(Stdio::null());
        match cmd.spawn() {
            Ok(mut child) => {
                thread::spawn(move || {
                    let _ = child.wait();
                });
                Response::empty_ok()
            }
            Err(err) => Response::ng(err.to_string()),
        }
    }

    /// 下载差异文件。没有总时长上限，只有停流看门狗。
    pub fn download_update(&self, mut options: DownloadOptions<'_>) -> Response<()> {
        let args = vec!["download_update".to_string()];
        let mut child = match self.command(&args).spawn() {
            Ok(child) => child,
            Err(err) => return Response::ng(err.to_string()),
        };

        let stdout = match child.stdout.take() {
            Some(stdout) => stdout,
            None => {
                let _ = child.kill();
                return Response::ng("failed to capture aly-client stdout");
            }
        };
        let stderr = child.stderr.take();
        let stderr_handle = stderr.map(|stream| {
            thread::spawn(move || {
                let mut buf = Vec::new();
                let mut stream = stream;
                let _ = stream.read_to_end(&mut buf);
                buf
            })
        });

        let (tx, rx) = mpsc::channel::<String>();
        let reader = thread::spawn(move || {
            let reader = BufReader::new(stdout);
            for line in reader.lines() {
                match line {
                    Ok(line) => {
                        if tx.send(line).is_err() {
                            return;
                        }
                    }
                    Err(_) => return,
                }
            }
        });

        let stall = if options.stall_timeout.is_zero() {
            DEFAULT_DOWNLOAD_STALL_TIMEOUT
        } else {
            options.stall_timeout
        };
        let mut last_output = Instant::now();

        loop {
            if let Some(token) = options.cancel {
                if token.is_cancelled() {
                    let _ = child.kill();
                    let _ = child.wait();
                    return Response::ng("download canceled");
                }
            }

            match rx.recv_timeout(Duration::from_millis(200)) {
                Ok(line) => {
                    last_output = Instant::now();
                    let trimmed = line.trim();
                    if trimmed.is_empty() {
                        continue;
                    }
                    let envelope: Envelope = match serde_json::from_str(trimmed) {
                        Ok(envelope) => envelope,
                        Err(err) => {
                            let _ = child.kill();
                            let _ = child.wait();
                            return Response::ng(format!("invalid progress line: {err}"));
                        }
                    };
                    if !envelope.is_success {
                        let message = if envelope.error_msg.is_empty() {
                            "download failed".to_string()
                        } else {
                            envelope.error_msg
                        };
                        let _ = child.kill();
                        let _ = child.wait();
                        return Response::ng(message);
                    }
                    match envelope.data {
                        None => {
                            let _ = child.wait();
                            return Response::empty_ok();
                        }
                        Some(value) => {
                            let progress: DownloadProgress =
                                match serde_json::from_value(value) {
                                    Ok(progress) => progress,
                                    Err(err) => {
                                        let _ = child.kill();
                                        let _ = child.wait();
                                        return Response::ng(format!(
                                            "invalid progress data: {err}"
                                        ));
                                    }
                                };
                            if progress.status == DownloadProgress::STATUS_DONE && progress.total > 0 {
                                if let Some(callback) = options.on_progress.as_mut() {
                                    callback(DownloadProgressInfo {
                                        file: progress.file,
                                        ratio: progress.index as f64 / progress.total as f64,
                                        status: progress.status,
                                        index: progress.index,
                                        total: progress.total,
                                        downloaded_bytes: progress.downloaded_bytes.unwrap_or(0),
                                        total_bytes: progress.total_bytes.unwrap_or(0),
                                        speed_bps: progress.speed_bps.unwrap_or(0),
                                    });
                                }
                            }
                        }
                    }
                }
                Err(RecvTimeoutError::Timeout) => {
                    if last_output.elapsed() > stall {
                        let _ = child.kill();
                        let _ = child.wait();
                        return Response::ng(format!(
                            "download stalled: no output for {}s, process killed",
                            stall.as_secs()
                        ));
                    }
                }
                Err(RecvTimeoutError::Disconnected) => {
                    let _ = child.wait();
                    let err_text = stderr_handle
                        .and_then(|handle| handle.join().ok())
                        .map(|buf| String::from_utf8_lossy(&buf).trim().to_string())
                        .unwrap_or_default();
                    let _ = reader.join();
                    return Response::ng(if err_text.is_empty() {
                        "download interrupted (aly-client exited before finish line)".to_string()
                    } else {
                        err_text
                    });
                }
            }
        }
    }

    fn run_simple<T: DeserializeOwned>(
        &self,
        command: &str,
        timeout: Duration,
        cancel: Option<&CancelToken>,
    ) -> Response<T> {
        let args = vec![command.to_string()];
        parse(self.run_collect(&args, timeout, cancel), timeout)
    }

    fn run_collect(
        &self,
        args: &[String],
        timeout: Duration,
        cancel: Option<&CancelToken>,
    ) -> RunResult {
        let mut child: Child = match self.command(args).spawn() {
            Ok(child) => child,
            Err(err) => {
                return RunResult {
                    start_error: Some(err.to_string()),
                    ..RunResult::default()
                }
            }
        };

        let stdout = child.stdout.take();
        let stderr = child.stderr.take();
        let out_handle = stdout.map(|mut stream| {
            thread::spawn(move || {
                let mut buf = Vec::new();
                let _ = stream.read_to_end(&mut buf);
                buf
            })
        });
        let err_handle = stderr.map(|mut stream| {
            thread::spawn(move || {
                let mut buf = Vec::new();
                let _ = stream.read_to_end(&mut buf);
                buf
            })
        });

        let deadline = Instant::now() + timeout;
        let mut timed_out = false;
        let mut canceled = false;
        loop {
            if let Some(token) = cancel {
                if token.is_cancelled() {
                    canceled = true;
                    let _ = child.kill();
                    break;
                }
            }
            if timeout > Duration::ZERO && Instant::now() >= deadline {
                timed_out = true;
                let _ = child.kill();
                break;
            }
            match child.try_wait() {
                Ok(Some(_)) => break,
                Ok(None) => thread::sleep(Duration::from_millis(20)),
                Err(_) => break,
            }
        }

        let exit_code = child
            .wait()
            .ok()
            .and_then(|status| status.code())
            .unwrap_or(-1);
        let stdout = out_handle.and_then(|handle| handle.join().ok()).unwrap_or_default();
        let stderr = err_handle.and_then(|handle| handle.join().ok()).unwrap_or_default();
        RunResult {
            stdout,
            stderr,
            code: exit_code,
            timed_out,
            canceled,
            start_error: None,
        }
    }
}

fn parse<T: DeserializeOwned>(result: RunResult, timeout: Duration) -> Response<T> {
    if let Some(err) = result.start_error {
        return Response::ng(err);
    }
    if result.canceled {
        return Response::ng("aly-client call canceled");
    }
    if result.timed_out {
        return Response::ng(format!(
            "aly-client timed out after {}s and was killed",
            timeout.as_secs()
        ));
    }

    let text = String::from_utf8_lossy(&result.stdout).trim().to_string();
    if text.is_empty() {
        let err_text = String::from_utf8_lossy(&result.stderr).trim().to_string();
        if result.code != 0 {
            return Response::ng(if err_text.is_empty() {
                format!(
                    "aly-client exited with code {} (may require admin)",
                    result.code
                )
            } else {
                err_text
            });
        }
        return Response::ng("aly-client returned no output (may require admin)");
    }

    match serde_json::from_str::<Envelope>(&text) {
        Ok(envelope) => {
            if !envelope.is_success {
                return Response {
                    is_success: false,
                    error_msg: envelope.error_msg,
                    data: None,
                };
            }
            match envelope.data {
                None => Response {
                    is_success: true,
                    error_msg: envelope.error_msg,
                    data: None,
                },
                Some(value) => match serde_json::from_value::<T>(value) {
                    Ok(data) => Response {
                        is_success: true,
                        error_msg: envelope.error_msg,
                        data: Some(data),
                    },
                    Err(err) => Response::ng(err.to_string()),
                },
            }
        }
        Err(err) => Response::ng(err.to_string()),
    }
}

#[derive(Deserialize)]
struct Envelope {
    #[serde(rename = "isSuccess")]
    is_success: bool,
    #[serde(rename = "errorMsg")]
    error_msg: String,
    data: Option<serde_json::Value>,
}

fn append_close_args(args: &mut Vec<String>, options: &CloseOptions) {
    if let Some(names) = &options.must_close_process_names {
        let names = names.trim();
        if !names.is_empty() {
            args.push("--must-close-process-name".to_string());
            args.push(names.to_string());
        }
    }
    if options.close_timeout_seconds != 0 && options.close_timeout_seconds != 30 {
        args.push("--close-timeout".to_string());
        args.push(options.close_timeout_seconds.to_string());
    }
}

#[derive(Default)]
struct RunResult {
    stdout: Vec<u8>,
    stderr: Vec<u8>,
    code: i32,
    timed_out: bool,
    canceled: bool,
    start_error: Option<String>,
}

#[cfg(windows)]
fn hide_window(cmd: &mut Command) {
    use std::os::windows::process::CommandExt;
    const CREATE_NO_WINDOW: u32 = 0x0800_0000;
    cmd.creation_flags(CREATE_NO_WINDOW);
}

#[cfg(not(windows))]
fn hide_window(_cmd: &mut Command) {}