use alyclient::{
    Api, CancelToken, CloseOptions, DownloadOptions, DownloadProgressInfo, Status, UpdateClient,
    DEFAULT_DOWNLOAD_STALL_TIMEOUT,
};

use std::path::PathBuf;
use std::sync::{Arc, Mutex, OnceLock};
use std::time::{Duration, Instant};

fn repo_root() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .ancestors()
        .nth(3)
        .expect("repo root")
        .to_path_buf()
}

fn fake_client() -> PathBuf {
    static FAKE: OnceLock<PathBuf> = OnceLock::new();
    FAKE.get_or_init(|| {
        let dir = std::env::temp_dir().join(format!("aly-rust-fake-{}", std::process::id()));
        std::fs::create_dir_all(&dir).expect("create fake dir");
        let name = if cfg!(windows) {
            "aly-client.exe"
        } else {
            "aly-client"
        };
        let out = dir.join(name);
        let status = std::process::Command::new("go")
            .arg("build")
            .arg("-o")
            .arg(&out)
            .arg(".")
            .current_dir(repo_root().join("tools").join("fake-aly-client"))
            .status()
            .expect("run go build");
        assert!(status.success(), "go build failed");
        out
    })
    .clone()
}

fn api(scenario: &str) -> Api {
    Api::new(fake_client()).with_env(
        "ALY_FAKE_SCENARIO",
        repo_root()
            .join("spec")
            .join("fixtures")
            .join("scenarios")
            .join(format!("{scenario}.json"))
            .to_string_lossy()
            .to_string(),
    )
}

fn wait_for_status(client: &UpdateClient, want: &str, timeout: Duration) {
    let deadline = Instant::now() + timeout;
    while Instant::now() < deadline {
        if client.status() == want {
            return;
        }
        std::thread::sleep(Duration::from_millis(20));
    }
    panic!(
        "wait_for_status timeout, last={}, error={}",
        client.status(),
        client.last_error()
    );
}

#[test]
fn check_update_available() {
    let response = api("update_available").check_update(None);
    assert!(response.is_success, "{}", response.error_msg);
    let data = response.data.expect("data");
    assert!(data.has_update);
    assert!(data.need_download_update);
    assert_eq!(data.new_version.as_deref(), Some("1.0.1"));
    assert_eq!(data.force_update, Some(false));
    assert_eq!(data.release_notes.expect("release notes").len(), 2);
}

#[test]
fn check_update_already_latest_and_offline() {
    for scenario in ["already_latest", "offline"] {
        let response = api(scenario).check_update(None);
        assert!(response.is_success, "{scenario}: {}", response.error_msg);
        assert!(!response.data.expect("data").has_update, "{scenario}");
    }
}

#[test]
fn download_progress_and_completion() {
    let mut progress: Vec<DownloadProgressInfo> = Vec::new();
    let response = api("update_available").download_update(DownloadOptions {
        stall_timeout: DEFAULT_DOWNLOAD_STALL_TIMEOUT,
        on_progress: Some(&mut |info| progress.push(info)),
        cancel: None,
    });
    assert!(response.is_success, "{}", response.error_msg);
    assert_eq!(progress.len(), 1);
    assert_eq!(progress[0].file, "app.exe");
    assert_eq!(progress[0].ratio, 1.0);
    assert_eq!(progress[0].status, "DONE");
    assert_eq!(progress[0].downloaded_bytes, 1000);
}

#[test]
fn download_partial_failure() {
    let response = api("download_partial_failure").download_update(DownloadOptions::new());
    assert!(!response.is_success);
    assert!(
        response.error_msg.contains("checksum mismatch"),
        "{}",
        response.error_msg
    );
}

#[test]
fn download_stall_watchdog() {
    let response = api("download_stall").download_update(DownloadOptions {
        stall_timeout: Duration::from_secs(1),
        on_progress: None,
        cancel: None,
    });
    assert!(!response.is_success);
    assert!(response.error_msg.contains("stalled"), "{}", response.error_msg);
}

#[test]
fn apply_update_returns_immediately() {
    let started = Instant::now();
    let response = api("update_available").apply_update(&CloseOptions::with_names("A,B"));
    assert!(response.is_success, "{}", response.error_msg);
    assert!(started.elapsed() < Duration::from_secs(10));
}

#[test]
fn self_update_flow() {
    let api = api("self_update");
    let check = api.check_self_update(None);
    assert!(check.is_success);
    assert!(check.data.expect("data").need_update);

    let updated = api.self_update(None);
    assert!(updated.is_success, "{}", updated.error_msg);
    let data = updated.data.expect("data");
    assert!(data.updated);
    assert_eq!(data.old_path.as_deref(), Some("aly-client.exe.old"));
}

#[test]
fn rollback_flow() {
    let api = api("rollback");
    let list = api.list_rollback_versions();
    assert!(list.is_success);
    assert_eq!(list.data.expect("data").versions.len(), 2);

    let rolled = api.rollback("1.0.0", &CloseOptions::default());
    assert!(rolled.is_success, "{}", rolled.error_msg);
    assert_eq!(rolled.data.expect("data").version, "1.0.0");
}

#[test]
fn check_diff_and_clean() {
    let diff = api("check_diff").check_diff();
    assert!(diff.is_success);
    let files = diff.data.expect("data").files;
    assert_eq!(files.len(), 1);
    assert_eq!(files[0].path, "app.exe");

    let cleaned = api("clean_all").clean(alyclient::CleanOptions {
        keep: None,
        all: true,
    });
    assert!(cleaned.is_success);
    assert_eq!(cleaned.data.expect("data").mode, "all");
}

#[test]
fn missing_executable_is_a_failed_response() {
    let missing = std::env::temp_dir()
        .join("aly-rust-missing")
        .join("aly-client.exe");
    let response = Api::new(missing).check_update(None);
    assert!(!response.is_success);
    assert!(!response.error_msg.is_empty());
}

#[test]
fn default_path_contains_update_folder() {
    assert!(Api::default_path().to_string_lossy().contains("UpdateFolder"));
}

#[test]
fn managed_force_update_runs_full_chain() {
    let statuses = Arc::new(Mutex::new(Vec::<String>::new()));
    let captured = Arc::clone(&statuses);
    let mut client = UpdateClient::new(api("force_update")).on_status(move |status, _| {
        captured.lock().unwrap().push(status.to_string());
    });

    client.start();
    wait_for_status(&client, Status::APPLY_UPDATE, Duration::from_secs(10));
    assert!(client.stop(Duration::from_secs(10)));

    let got = statuses.lock().unwrap().clone();
    assert_eq!(got.first().map(String::as_str), Some(Status::DISCOVERED_UPDATE));
    assert!(got.iter().any(|s| s == Status::DOWNLOADING_UPDATE));
    assert!(got.iter().any(|s| s == Status::DOWNLOADED_UPDATE));
    assert_eq!(got.last().map(String::as_str), Some(Status::APPLY_UPDATE));
    assert_eq!(client.last_error(), "");
    assert!(!client.is_running());
}

#[test]
fn managed_non_force_waits_for_confirmations() {
    let downloads = Arc::new(Mutex::new(Vec::<String>::new()));
    let applies = Arc::new(Mutex::new(Vec::<String>::new()));
    let d = Arc::clone(&downloads);
    let a = Arc::clone(&applies);

    let mut client = UpdateClient::new(api("update_available"))
        .confirm_download(move |version, _| d.lock().unwrap().push(version.to_string()))
        .confirm_apply(move |version, _| a.lock().unwrap().push(version.to_string()));

    client.start();
    wait_for_status(&client, Status::APPLY_UPDATE, Duration::from_secs(10));
    assert!(client.stop(Duration::from_secs(10)));

    assert_eq!(downloads.lock().unwrap().clone(), vec!["1.0.1".to_string()]);
    assert_eq!(applies.lock().unwrap().clone(), vec!["1.0.1".to_string()]);
}

#[test]
fn managed_offline_stays_idle() {
    let mut client = UpdateClient::new(api("offline"));
    client.start();
    std::thread::sleep(Duration::from_millis(500));
    assert_eq!(client.status(), Status::NONE);
    assert!(client.stop(Duration::from_secs(10)));
}

#[test]
fn stop_interrupts_pending_confirmation() {
    let mut client = UpdateClient::new(api("update_available")).confirm_download(|_, token| {
        while !token.is_cancelled() {
            std::thread::sleep(Duration::from_millis(10));
        }
    });

    client.start();
    wait_for_status(&client, Status::DISCOVERED_UPDATE, Duration::from_secs(10));

    let started = Instant::now();
    assert!(client.stop(Duration::from_secs(10)));
    assert!(started.elapsed() < Duration::from_secs(2));
}

#[test]
fn stop_is_idempotent() {
    let mut client = UpdateClient::new(api("offline"));
    client.start();
    std::thread::sleep(Duration::from_millis(100));
    assert!(client.stop(Duration::from_secs(10)));
    assert!(client.stop(Duration::from_secs(10)));
    assert!(!client.is_running());
}

#[test]
fn cancel_token_behavior() {
    let token = CancelToken::new();
    assert!(!token.is_cancelled());
    token.cancel();
    assert!(token.is_cancelled());
}
