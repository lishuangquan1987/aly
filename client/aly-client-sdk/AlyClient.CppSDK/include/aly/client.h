#pragma once

#include "api.h"

#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN
#endif
#include <windows.h>

#include <atomic>
#include <functional>
#include <memory>
#include <string>

namespace aly {

namespace Status {
static const char* const NONE = "none";
static const char* const DISCOVERED_UPDATE = "discoveredUpdate";
static const char* const DOWNLOADING_UPDATE = "downloadingUpdate";
static const char* const DOWNLOADED_UPDATE = "downloadedUpdate";
static const char* const APPLY_UPDATE = "applyUpdate";
}

/// aly-client 的托管生命周期：自更新 → check → download → apply。
///
/// 确认回调在循环线程内同步执行，签名 (version, cancelToken)；返回即确认。
/// 阻塞时应轮询 token，保证 stop 能及时退出。
class UpdateClient {
public:
    typedef std::function<void(const std::string&, const std::string&)> StatusCallback;
    typedef std::function<void(const std::string&)> ErrorCallback;
    typedef std::function<void(const std::string&, const CancelToken&)> ConfirmCallback;

    explicit UpdateClient(const Api& api);
    ~UpdateClient();

    void setOnStatus(const StatusCallback& callback);
    void setOnError(const ErrorCallback& callback);
    void setConfirmDownload(const ConfirmCallback& callback);
    void setConfirmApply(const ConfirmCallback& callback);

    void setMustCloseProcessNames(const std::string& names);
    void setCloseTimeoutSeconds(int seconds);
    void setIdlePollSeconds(int seconds);

    /// 启动后台循环（立即返回）。一个实例只启动一次。
    void start();

    /// 停止轮询并等待循环退出；返回是否在超时内退出。
    bool stop(int timeoutMs = 5000);

    std::string status() const;
    std::string lastError() const;
    bool isRunning() const;

private:
    static DWORD WINAPI threadProc(LPVOID parameter);
    void run();
    void setStatus(const std::string& status, const std::string& message);
    void reportError(const std::string& message);
    void clearError();
    bool sleepFor(int milliseconds);
    int idleDelayMs() const;

    Api api_;
    std::atomic<bool> running_;
    std::shared_ptr<std::atomic<bool> > cancelled_;
    HANDLE thread_;

    mutable CRITICAL_SECTION lock_;
    std::string status_;
    std::string error_;

    StatusCallback onStatus_;
    ErrorCallback onError_;
    ConfirmCallback confirmDownload_;
    ConfirmCallback confirmApply_;

    std::string mustCloseProcessNames_;
    int closeTimeoutSeconds_;
    int idlePollSeconds_;
};

} // namespace aly