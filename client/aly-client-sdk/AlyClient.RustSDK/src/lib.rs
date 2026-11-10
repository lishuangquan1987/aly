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
}

impl<T> Response<T> {
    pub fn ok(data: T) -> Self {
        Self {
            is_success: true,
            error_msg: String::new(),
            data: Some(data),
        }
    }

    pub fn ng(error_msg: impl Into<String>) -> Self {
        Self {
            is_success: false,
            error_msg: error_msg.into(),
            data: None,
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
        }
    }
}