#pragma once

#include "models_gen.h"

#include <atomic>
#include <functional>
#include <memory>
#include <string>
#include <vector>

namespace aly {

template <typename T>
struct Response {
    bool isSuccess;
    std::string errorMsg;
    Optional<T> data;
    /// download_update 失败终态的原因机器码（协议 #13）：
    /// already_latest / partial_failure / materialize_failed / version_state_failed。
    /// 机器判定优先于 errorMsg 文案匹配；空串表示未知/旧版客户端。
    std::string reason;

    Response() : isSuccess(false), errorMsg(), data(), reason() {}

    /// 便捷判定：下载失败是否"已是最新"（#13：reason 优先，文案匹配兼容回退）。
    bool isAlreadyLatest() const {
        if (reason == "already_latest") {
            return true;
        }
        return reason.empty() && errorMsg.find("already at latest version") != std::string::npos;
    }
};

/// 取消令牌；生命周期停止时 cancel()。
class CancelToken {
public:
    CancelToken();
    explicit CancelToken(const std::shared_ptr<std::atomic<bool> >& flag);

    void cancel() const;
    bool isCancelled() const;

private:
    std::shared_ptr<std::atomic<bool> > flag_;
};

struct CloseOptions {
    std::string mustCloseProcessNames;
    int closeTimeoutSeconds;

    CloseOptions() : mustCloseProcessNames(), closeTimeoutSeconds(30) {}
    static CloseOptions withNames(const std::string& names);
};

struct CleanOptions {
    int keep;
    bool all;

    CleanOptions() : keep(0), all(false) {}
};

struct DownloadProgressInfo {
    std::string file;
    double ratio;
    std::string status;
    long long index;
    long long total;
    long long downloadedBytes;
    long long totalBytes;
    long long speedBps;
};

struct DownloadOptions {
    int stallTimeoutMs;
    std::function<void(const DownloadProgressInfo&)> onProgress;
    const CancelToken* cancel;

    DownloadOptions() : stallTimeoutMs(1800000), onProgress(), cancel(NULL) {}
};

/// 封装一个 aly-client 可执行文件（Windows XP+）。
class Api {
public:
    explicit Api(const std::string& path);

    Api& withEnv(const std::string& key, const std::string& value);

    const std::string& path() const { return path_; }

    /// 默认更新器路径：<宿主 exe 目录>/../UpdateFolder/aly-client.exe。
    static std::string defaultPath();

    Response<CheckUpdateData> checkUpdate(const CancelToken* cancel = NULL) const;
    Response<CheckSelfUpdateData> checkSelfUpdate(const CancelToken* cancel = NULL) const;
    Response<SelfUpdateData> selfUpdate(const CancelToken* cancel = NULL) const;
    Response<RollbackListData> listRollbackVersions() const;
    Response<CheckDiffData> checkDiff() const;
    Response<CleanData> clean(const CleanOptions& options = CleanOptions()) const;
    Response<RollbackData> rollback(
        const std::string& version, const CloseOptions& options = CloseOptions()) const;

    /// 拉起独立更新进程后立即返回，不等待。
    Response<bool> applyUpdate(const CloseOptions& options = CloseOptions()) const;

    /// 下载差异文件。没有总时长上限，只有停流看门狗。
    Response<bool> downloadUpdate(const DownloadOptions& options = DownloadOptions()) const;

private:
    std::string path_;
    std::vector<std::pair<std::string, std::string> > env_;
};

} // namespace aly