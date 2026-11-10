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

    Response() : isSuccess(false), errorMsg(), data() {}
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