#include "aly/client.h"

#include <cstdio>

namespace aly {

UpdateClient::UpdateClient(const Api& api)
    : api_(api),
      running_(false),
      cancelled_(std::make_shared<std::atomic<bool> >(false)),
      thread_(NULL),
      status_(Status::NONE),
      error_(),
      closeTimeoutSeconds_(30),
      idlePollSeconds_(300) {
    InitializeCriticalSection(&lock_);
}

UpdateClient::~UpdateClient() {
    stop(5000);
    if (thread_ != NULL) {
        CloseHandle(thread_);
        thread_ = NULL;
    }
    DeleteCriticalSection(&lock_);
}

void UpdateClient::setOnStatus(const StatusCallback& callback) {
    onStatus_ = callback;
}

void UpdateClient::setOnError(const ErrorCallback& callback) {
    onError_ = callback;
}

void UpdateClient::setConfirmDownload(const ConfirmCallback& callback) {
    confirmDownload_ = callback;
}

void UpdateClient::setConfirmApply(const ConfirmCallback& callback) {
    confirmApply_ = callback;
}

void UpdateClient::setMustCloseProcessNames(const std::string& names) {
    mustCloseProcessNames_ = names;
}

void UpdateClient::setCloseTimeoutSeconds(int seconds) {
    closeTimeoutSeconds_ = seconds;
}

void UpdateClient::setIdlePollSeconds(int seconds) {
    idlePollSeconds_ = seconds;
}

void UpdateClient::start() {
    if (thread_ != NULL) {
        if (WaitForSingleObject(thread_, 0) == WAIT_TIMEOUT) {
            return;
        }
        CloseHandle(thread_);
        thread_ = NULL;
    }
    running_.store(true, std::memory_order_seq_cst);
    cancelled_->store(false, std::memory_order_seq_cst);
    thread_ = CreateThread(NULL, 0, &UpdateClient::threadProc, this, 0, NULL);
    if (thread_ == NULL) {
        running_.store(false, std::memory_order_seq_cst);
        reportError("CreateThread failed");
    }
}

bool UpdateClient::stop(int timeoutMs) {
    running_.store(false, std::memory_order_seq_cst);
    cancelled_->store(true, std::memory_order_seq_cst);
    if (thread_ == NULL) {
        return true;
    }
    DWORD wait = WaitForSingleObject(thread_, timeoutMs < 0 ? 0 : static_cast<DWORD>(timeoutMs));
    if (wait == WAIT_OBJECT_0) {
        CloseHandle(thread_);
        thread_ = NULL;
        return true;
    }
    return false;
}

std::string UpdateClient::status() const {
    EnterCriticalSection(&lock_);
    std::string value = status_;
    LeaveCriticalSection(&lock_);
    return value;
}

std::string UpdateClient::lastError() const {
    EnterCriticalSection(&lock_);
    std::string value = error_;
    LeaveCriticalSection(&lock_);
    return value;
}

bool UpdateClient::isRunning() const {
    return thread_ != NULL && WaitForSingleObject(thread_, 0) == WAIT_TIMEOUT;
}

DWORD WINAPI UpdateClient::threadProc(LPVOID parameter) {
    static_cast<UpdateClient*>(parameter)->run();
    return 0;
}

void UpdateClient::run() {
    CancelToken token(cancelled_);

    Response<CheckSelfUpdateData> selfCheck = api_.checkSelfUpdate(&token);
    if (!token.isCancelled() && selfCheck.isSuccess && selfCheck.data.has_value()
        && selfCheck.data.get().need_update) {
        Response<SelfUpdateData> updated = api_.selfUpdate(&token);
        if (!updated.isSuccess) {
            reportError(updated.errorMsg);
        }
    }

    while (running_.load(std::memory_order_seq_cst) && !token.isCancelled()) {
        Response<CheckUpdateData> check = api_.checkUpdate(&token);
        if (!running_.load(std::memory_order_seq_cst) || token.isCancelled()) {
            return;
        }
        if (!check.isSuccess) {
            reportError(check.errorMsg);
            if (!sleepFor(5000)) {
                return;
            }
            continue;
        }
        clearError();

        if (!check.data.has_value()) {
            reportError("check_update returned no data");
            if (!sleepFor(5000)) {
                return;
            }
            continue;
        }
        const CheckUpdateData& data = check.data.get();
        if (!data.has_update || !data.need_download_update) {
            if (status() != Status::NONE) {
                setStatus(Status::NONE, "Update status cleared");
            }
            if (!sleepFor(idleDelayMs())) {
                return;
            }
            continue;
        }

        bool force = data.force_update.has_value() && data.force_update.get();
        std::string version = data.new_version.value_or(std::string());
        setStatus(
            Status::DISCOVERED_UPDATE,
            version.empty() ? "Found new version" : "Found new version " + version);
        if (!force) {
            if (confirmDownload_) {
                try {
                    confirmDownload_(version, token);
                } catch (...) {
                    reportError("confirm handler error");
                }
            }
            if (!running_.load(std::memory_order_seq_cst) || token.isCancelled()) {
                return;
            }
        }

        setStatus(Status::DOWNLOADING_UPDATE, "Downloading update...");
        DownloadOptions options;
        options.stallTimeoutMs = 1800000;
        options.cancel = &token;
        options.onProgress = [this](const DownloadProgressInfo& info) {
            char message[256];
            std::snprintf(
                message,
                sizeof(message),
                "Downloading %s... %d%%",
                info.file.c_str(),
                static_cast<int>(info.ratio * 100.0));
            setStatus(Status::DOWNLOADING_UPDATE, message);
        };
        Response<bool> downloaded = api_.downloadUpdate(options);
        if (!running_.load(std::memory_order_seq_cst) || token.isCancelled()) {
            return;
        }
        if (!downloaded.isSuccess) {
            if (downloaded.errorMsg.find("already at latest version") != std::string::npos) {
                setStatus(Status::NONE, "Update status cleared");
                if (!sleepFor(idleDelayMs())) {
                    return;
                }
                continue;
            }
            reportError(downloaded.errorMsg);
            if (!sleepFor(1000)) {
                return;
            }
            continue;
        }

        setStatus(Status::DOWNLOADED_UPDATE, "Update downloaded");
        if (!force) {
            if (confirmApply_) {
                try {
                    confirmApply_(version, token);
                } catch (...) {
                    reportError("confirm handler error");
                }
            }
            if (!running_.load(std::memory_order_seq_cst) || token.isCancelled()) {
                return;
            }
        }

        setStatus(Status::APPLY_UPDATE, "Applying update...");
        CloseOptions close;
        close.mustCloseProcessNames = mustCloseProcessNames_;
        close.closeTimeoutSeconds = closeTimeoutSeconds_;
        Response<bool> applied = api_.applyUpdate(close);
        if (!applied.isSuccess) {
            reportError("apply spawn failed: " + applied.errorMsg);
        }
        if (!sleepFor(30000)) {
            return;
        }
    }
}

void UpdateClient::setStatus(const std::string& status, const std::string& message) {
    StatusCallback callback;
    EnterCriticalSection(&lock_);
    status_ = status;
    callback = onStatus_;
    LeaveCriticalSection(&lock_);
    if (callback) {
        try {
            callback(status, message);
        } catch (...) {
            // 回调异常不得打断循环
        }
    }
}

void UpdateClient::reportError(const std::string& message) {
    ErrorCallback callback;
    {
        EnterCriticalSection(&lock_);
        if (error_ == message) {
            LeaveCriticalSection(&lock_);
            return;
        }
        error_ = message;
        callback = onError_;
        LeaveCriticalSection(&lock_);
    }
    if (callback) {
        try {
            callback(message);
        } catch (...) {
            // 回调异常不得打断循环
        }
    }
}

void UpdateClient::clearError() {
    ErrorCallback callback;
    {
        EnterCriticalSection(&lock_);
        if (error_.empty()) {
            LeaveCriticalSection(&lock_);
            return;
        }
        error_.clear();
        callback = onError_;
        LeaveCriticalSection(&lock_);
    }
    if (callback) {
        try {
            callback(std::string());
        } catch (...) {
            // ignore
        }
    }
}

bool UpdateClient::sleepFor(int milliseconds) {
    DWORD started = GetTickCount();
    while (running_.load(std::memory_order_seq_cst)) {
        if (static_cast<int>(GetTickCount() - started) >= milliseconds) {
            break;
        }
        Sleep(50);
    }
    return running_.load(std::memory_order_seq_cst);
}

int UpdateClient::idleDelayMs() const {
    int seconds = idlePollSeconds_;
    if (seconds <= 0) {
        seconds = 300;
    }
    if (seconds < 60) {
        seconds = 60;
    }
    int jitter = static_cast<int>(GetTickCount() % 61);
    return (seconds + jitter) * 1000;
}

} // namespace aly