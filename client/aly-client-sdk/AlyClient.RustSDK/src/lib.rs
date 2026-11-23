//! Aly auto-update client SDK for Rust.
//!
//! Wraps `aly-client` and provides typed models from `spec/aly-protocol.schema.json`,
//! a blocking API layer, and a managed update lifecycle.

mod api;
mod client;
mod models_gen;

pub use api::{
    Api, CancelToken, CleanOptions, CloseOptions, DownloadOptions, DownloadProgressInfo,
    DEFAULT_DOWNLOAD_STALL_TIMEOUT, DEFAULT_SHORT_TIMEOUT, ROLLBACK_TIMEOUT,
};
pub use client::{Status, UpdateClient};
pub use models_gen::*;

use serde::Deserialize;

/// 所有命令的统一响应；业务失败不会返回 `Err`。
#[derive(Debug, Clone, Deserialize)]
pub struct Response<T> {
    #[serde(rename = "isSuccess")]
    pub is_success: bool,
    #[serde(rename = "errorMsg")]
    pub error_msg: String,
    pub data: Option<T>,
    /// download_update 失败终态的原因机器码（协议 #13）：
    /// already_latest / partial_failure / materialize_failed / version_state_failed。
    /// 机器判定优先于 error_msg 文案匹配。
    #[serde(skip)]
    pub reason: Option<String>,
}

impl<T> Response<T> {
    pub fn ok(data: T) -> Self {
        Self {
            is_success: true,
            error_msg: String::new(),
            data: Some(data),
            reason: None,
        }
    }

    pub fn ng(error_msg: impl Into<String>) -> Self {
        Self {
            is_success: false,
            error_msg: error_msg.into(),
            data: None,
            reason: None,
        }
    }

    /// 失败响应 + reason 机器码（协议 #13）。
    pub fn ng_with_reason(error_msg: impl Into<String>, reason: Option<String>) -> Self {
        Self {
            is_success: false,
            error_msg: error_msg.into(),
            data: None,
            reason,
        }
    }

    /// 便捷判定：下载失败是否"已是最新"（#13：reason 优先，文案匹配兼容回退）。
    pub fn is_already_latest(&self) -> bool {
        match &self.reason {
            Some(r) => r == "already_latest",
            None => self.error_msg.contains("already at latest version"),
        }
    }
}

impl Response<()> {
    /// 无 data 载荷的成功响应（apply_update / download_update 完成）。
    pub fn empty_ok() -> Self {
        Self {
            is_success: true,
            error_msg: String::new(),
            data: None,
            reason: None,
        }
    }
}