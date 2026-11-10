#include "aly/api.h"

#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN
#endif
#include <windows.h>

#include <algorithm>
#include <cstdio>
#include <cstdlib>
#include <deque>
#include <stdexcept>

namespace aly {
namespace {

const int kShortTimeoutMs = 30000;
const int kRollbackTimeoutMs = 600000;

std::string winError(DWORD code) {
    wchar_t* buffer = NULL;
    DWORD length = FormatMessageW(
        FORMAT_MESSAGE_ALLOCATE_BUFFER | FORMAT_MESSAGE_FROM_SYSTEM | FORMAT_MESSAGE_IGNORE_INSERTS,
        NULL,
        code,
        MAKELANGID(LANG_NEUTRAL, SUBLANG_DEFAULT),
        reinterpret_cast<LPWSTR>(&buffer),
        0,
        NULL);
    std::string message;
    if (length != 0 && buffer != NULL) {
        std::wstring wide(buffer, length);
        int size = WideCharToMultiByte(CP_UTF8, 0, wide.c_str(), -1, NULL, 0, NULL, NULL);
        if (size > 1) {
            std::string utf8(static_cast<size_t>(size - 1), '\0');
            WideCharToMultiByte(CP_UTF8, 0, wide.c_str(), -1, &utf8[0], size, NULL, NULL);
            message = utf8;
        }
        LocalFree(buffer);
    }
    while (!message.empty() && (message[message.size() - 1] == '\r' || message[message.size() - 1] == '\n')) {
        message.erase(message.size() - 1);
    }
    char codeText[32];
    std::snprintf(codeText, sizeof(codeText), " (error %lu)", static_cast<unsigned long>(code));
    return message.empty() ? std::string("Windows error") + codeText : message + codeText;
}

std::wstring utf8ToWide(const std::string& text) {
    if (text.empty()) {
        return std::wstring();
    }
    int size = MultiByteToWideChar(CP_UTF8, 0, text.c_str(), -1, NULL, 0);
    if (size <= 1) {
        throw std::runtime_error("invalid UTF-8 string");
    }
    std::wstring wide(static_cast<size_t>(size - 1), L'\0');
    MultiByteToWideChar(CP_UTF8, 0, text.c_str(), -1, &wide[0], size);
    return wide;
}

std::string wideToUtf8(const std::wstring& wide) {
    if (wide.empty()) {
        return std::string();
    }
    int size = WideCharToMultiByte(CP_UTF8, 0, wide.c_str(), -1, NULL, 0, NULL, NULL);
    if (size <= 1) {
        return std::string();
    }
    std::string utf8(static_cast<size_t>(size - 1), '\0');
    WideCharToMultiByte(CP_UTF8, 0, wide.c_str(), -1, &utf8[0], size, NULL, NULL);
    return utf8;
}

std::wstring fullPath(const std::wstring& path) {
    std::vector<wchar_t> buffer(MAX_PATH);
    for (;;) {
        DWORD length = GetFullPathNameW(path.c_str(), static_cast<DWORD>(buffer.size()), &buffer[0], NULL);
        if (length == 0) {
            throw std::runtime_error("GetFullPathName failed: " + winError(GetLastError()));
        }
        if (length < buffer.size()) {
            return std::wstring(&buffer[0], length);
        }
        buffer.resize(length + 1);
    }
}

std::wstring parentDir(const std::wstring& path) {
    size_t slash = path.find_last_of(L"\\/");
    if (slash == std::wstring::npos) {
        return std::wstring();
    }
    return path.substr(0, slash);
}

std::wstring quoteArg(const std::wstring& arg) {
    if (!arg.empty() && arg.find_first_of(L" \t\"") == std::wstring::npos) {
        return arg;
    }
    std::wstring out = L"\"";
    size_t backslashes = 0;
    for (size_t i = 0; i < arg.size(); ++i) {
        wchar_t c = arg[i];
        if (c == L'\\') {
            ++backslashes;
        } else if (c == L'"') {
            out.append(backslashes * 2 + 1, L'\\');
            out.push_back(L'"');
            backslashes = 0;
        } else {
            out.append(backslashes, L'\\');
            out.push_back(c);
            backslashes = 0;
        }
    }
    out.append(backslashes * 2, L'\\');
    out.push_back(L'"');
    return out;
}

std::wstring buildCommandLine(const std::wstring& exe, const std::vector<std::string>& args) {
    std::wstring command = quoteArg(exe);
    for (size_t i = 0; i < args.size(); ++i) {
        command.push_back(L' ');
        command += quoteArg(utf8ToWide(args[i]));
    }
    return command;
}

std::wstring buildEnvironment(const std::vector<std::pair<std::string, std::string> >& extra) {
    std::vector<std::wstring> entries;
    LPWCH block = GetEnvironmentStringsW();
    if (block != NULL) {
        const wchar_t* cursor = block;
        while (*cursor != L'\0') {
            std::wstring entry(cursor);
            entries.push_back(entry);
            cursor += entry.size() + 1;
        }
        FreeEnvironmentStringsW(block);
    }
    for (size_t i = 0; i < extra.size(); ++i) {
        std::wstring key = utf8ToWide(extra[i].first);
        std::wstring prefix = key + L"=";
        for (size_t j = 0; j < entries.size();) {
            if (entries[j].size() >= prefix.size()
                && _wcsnicmp(entries[j].c_str(), prefix.c_str(), prefix.size()) == 0) {
                entries.erase(entries.begin() + static_cast<std::ptrdiff_t>(j));
            } else {
                ++j;
            }
        }
        entries.push_back(key + L"=" + utf8ToWide(extra[i].second));
    }
    std::sort(entries.begin(), entries.end(), [](const std::wstring& a, const std::wstring& b) {
        return _wcsicmp(a.c_str(), b.c_str()) < 0;
    });
    std::wstring blockOut;
    for (size_t i = 0; i < entries.size(); ++i) {
        blockOut += entries[i];
        blockOut.push_back(L'\0');
    }
    blockOut.push_back(L'\0');
    return blockOut;
}

HANDLE openNul(bool inherit) {
    SECURITY_ATTRIBUTES sa;
    sa.nLength = sizeof(sa);
    sa.lpSecurityDescriptor = NULL;
    sa.bInheritHandle = inherit ? TRUE : FALSE;
    return CreateFileW(
        L"NUL",
        GENERIC_READ | GENERIC_WRITE,
        FILE_SHARE_READ | FILE_SHARE_WRITE,
        &sa,
        OPEN_EXISTING,
        FILE_ATTRIBUTE_NORMAL,
        NULL);
}

struct Spawned {
    HANDLE process;
    HANDLE thread;
    HANDLE stdoutRead;
    HANDLE stderrRead;
    std::string error;

    Spawned() : process(NULL), thread(NULL), stdoutRead(NULL), stderrRead(NULL), error() {}
};

Spawned spawnProcess(
    const std::string& clientPath,
    const std::vector<std::string>& args,
    const std::vector<std::pair<std::string, std::string> >& env,
    bool capture) {
    Spawned result;
    HANDLE stdoutRead = NULL;
    HANDLE stdoutWrite = NULL;
    HANDLE stderrRead = NULL;
    HANDLE stderrWrite = NULL;
    HANDLE stdinNul = NULL;

    SECURITY_ATTRIBUTES pipeSa;
    pipeSa.nLength = sizeof(pipeSa);
    pipeSa.lpSecurityDescriptor = NULL;
    pipeSa.bInheritHandle = TRUE;

    if (capture) {
        if (!CreatePipe(&stdoutRead, &stdoutWrite, &pipeSa, 0)
            || !CreatePipe(&stderrRead, &stderrWrite, &pipeSa, 0)) {
            result.error = "CreatePipe failed: " + winError(GetLastError());
            if (stdoutRead) CloseHandle(stdoutRead);
            if (stdoutWrite) CloseHandle(stdoutWrite);
            if (stderrRead) CloseHandle(stderrRead);
            if (stderrWrite) CloseHandle(stderrWrite);
            return result;
        }
        SetHandleInformation(stdoutRead, HANDLE_FLAG_INHERIT, 0);
        SetHandleInformation(stderrRead, HANDLE_FLAG_INHERIT, 0);
    }
    stdinNul = openNul(true);
    if (stdinNul == INVALID_HANDLE_VALUE) {
        result.error = "open NUL failed: " + winError(GetLastError());
        if (stdoutRead) CloseHandle(stdoutRead);
        if (stdoutWrite) CloseHandle(stdoutWrite);
        if (stderrRead) CloseHandle(stderrRead);
        if (stderrWrite) CloseHandle(stderrWrite);
        return result;
    }

    std::wstring exe = fullPath(utf8ToWide(clientPath));
    std::wstring workdir = parentDir(exe);
    std::wstring command = buildCommandLine(exe, args);
    std::wstring envBlock = buildEnvironment(env);
    std::vector<wchar_t> commandBuffer(command.begin(), command.end());
    commandBuffer.push_back(L'\0');

    STARTUPINFOW si;
    ZeroMemory(&si, sizeof(si));
    si.cb = sizeof(si);
    si.dwFlags = STARTF_USESTDHANDLES;
    si.hStdInput = stdinNul;
    si.hStdOutput = capture ? stdoutWrite : stdinNul;
    si.hStdError = capture ? stderrWrite : stdinNul;

    PROCESS_INFORMATION pi;
    ZeroMemory(&pi, sizeof(pi));
    DWORD flags = CREATE_NO_WINDOW | CREATE_UNICODE_ENVIRONMENT;
    BOOL ok = CreateProcessW(
        exe.c_str(),
        &commandBuffer[0],
        NULL,
        NULL,
        TRUE,
        flags,
        &envBlock[0],
        workdir.empty() ? NULL : workdir.c_str(),
        &si,
        &pi);
    if (!ok) {
        result.error = "CreateProcess failed: " + winError(GetLastError());
    } else {
        result.process = pi.hProcess;
        result.thread = pi.hThread;
        result.stdoutRead = stdoutRead;
        result.stderrRead = stderrRead;
        stdoutRead = NULL;
        stderrRead = NULL;
    }

    if (stdoutRead) CloseHandle(stdoutRead);
    if (stdoutWrite) CloseHandle(stdoutWrite);
    if (stderrRead) CloseHandle(stderrRead);
    if (stderrWrite) CloseHandle(stderrWrite);
    CloseHandle(stdinNul);
    return result;
}

struct ReaderContext {
    HANDLE pipe;
    std::string data;
};

DWORD WINAPI readerProc(LPVOID parameter) {
    ReaderContext* context = static_cast<ReaderContext*>(parameter);
    char buffer[4096];
    DWORD read = 0;
    while (ReadFile(context->pipe, buffer, sizeof(buffer), &read, NULL) && read > 0) {
        context->data.append(buffer, read);
    }
    return 0;
}

struct RunResult {
    std::string out;
    std::string err;
    DWORD code;
    bool timedOut;
    bool canceled;
    std::string startError;

    RunResult() : code(0), timedOut(false), canceled(false), startError() {}
};

RunResult runCollect(
    const std::string& clientPath,
    const std::vector<std::string>& args,
    const std::vector<std::pair<std::string, std::string> >& env,
    int timeoutMs,
    const CancelToken* cancel) {
    RunResult result;
    Spawned process = spawnProcess(clientPath, args, env, true);
    if (!process.error.empty()) {
        result.startError = process.error;
        return result;
    }

    ReaderContext stdoutContext;
    stdoutContext.pipe = process.stdoutRead;
    ReaderContext stderrContext;
    stderrContext.pipe = process.stderrRead;
    HANDLE stdoutThread = CreateThread(NULL, 0, readerProc, &stdoutContext, 0, NULL);
    HANDLE stderrThread = CreateThread(NULL, 0, readerProc, &stderrContext, 0, NULL);

    DWORD started = GetTickCount();
    for (;;) {
        DWORD wait = WaitForSingleObject(process.process, 20);
        if (wait == WAIT_OBJECT_0) {
            break;
        }
        if (cancel != NULL && cancel->isCancelled()) {
            result.canceled = true;
            TerminateProcess(process.process, 1);
            break;
        }
        if (timeoutMs > 0 && static_cast<int>(GetTickCount() - started) >= timeoutMs) {
            result.timedOut = true;
            TerminateProcess(process.process, 1);
            break;
        }
    }

    WaitForSingleObject(process.process, INFINITE);
    DWORD code = 0;
    GetExitCodeProcess(process.process, &code);
    result.code = code;
    if (stdoutThread) {
        WaitForSingleObject(stdoutThread, INFINITE);
        CloseHandle(stdoutThread);
    }
    if (stderrThread) {
        WaitForSingleObject(stderrThread, INFINITE);
        CloseHandle(stderrThread);
    }
    result.out = stdoutContext.data;
    result.err = stderrContext.data;
    CloseHandle(process.stdoutRead);
    CloseHandle(process.stderrRead);
    CloseHandle(process.process);
    CloseHandle(process.thread);
    return result;
}

std::string trim(const std::string& text) {
    size_t start = 0;
    while (start < text.size() && (text[start] == ' ' || text[start] == '\t' || text[start] == '\r'
                                   || text[start] == '\n')) {
        ++start;
    }
    size_t end = text.size();
    while (end > start && (text[end - 1] == ' ' || text[end - 1] == '\t' || text[end - 1] == '\r'
                           || text[end - 1] == '\n')) {
        --end;
    }
    return text.substr(start, end - start);
}

template <typename T>
Response<T> ng(const std::string& message) {
    Response<T> response;
    response.isSuccess = false;
    response.errorMsg = message;
    return response;
}

template <typename T>
Response<T> ok(const T& data) {
    Response<T> response;
    response.isSuccess = true;
    response.data = Optional<T>(data);
    return response;
}

template <typename T>
Response<T> parseRun(const RunResult& run, int timeoutMs, void (*fromJson)(const JsonValue&, T&)) {
    if (!run.startError.empty()) {
        return ng<T>(run.startError);
    }
    if (run.canceled) {
        return ng<T>("aly-client call canceled");
    }
    if (run.timedOut) {
        char message[128];
        std::snprintf(
            message, sizeof(message), "aly-client timed out after %ds and was killed", timeoutMs / 1000);
        return ng<T>(message);
    }
    std::string text = trim(run.out);
    if (text.empty()) {
        std::string err = trim(run.err);
        if (run.code != 0) {
            if (!err.empty()) {
                return ng<T>(err);
            }
            char message[128];
            std::snprintf(
                message,
                sizeof(message),
                "aly-client exited with code %lu (may require admin)",
                static_cast<unsigned long>(run.code));
            return ng<T>(message);
        }
        return ng<T>("aly-client returned no output (may require admin)");
    }
    try {
        JsonValue root = parseJson(text);
        Response<T> response;
        response.isSuccess = jsonBool(root, "isSuccess", false);
        response.errorMsg = jsonString(root, "errorMsg");
        if (!response.isSuccess) {
            return response;
        }
        if (root.has("data") && !root["data"].isNull()) {
            T value;
            fromJson(root["data"], value);
            response.data = Optional<T>(value);
        }
        return response;
    } catch (const std::exception& ex) {
        return ng<T>(ex.what());
    }
}

void appendCloseArgs(std::vector<std::string>& args, const CloseOptions& options) {
    if (!options.mustCloseProcessNames.empty()) {
        args.push_back("--must-close-process-name");
        args.push_back(options.mustCloseProcessNames);
    }
    if (options.closeTimeoutSeconds != 0 && options.closeTimeoutSeconds != 30) {
        args.push_back("--close-timeout");
        char value[32];
        std::snprintf(value, sizeof(value), "%d", options.closeTimeoutSeconds);
        args.push_back(value);
    }
}

class LineQueue {
public:
    LineQueue() : done_(false) { InitializeCriticalSection(&lock_); }
    ~LineQueue() { DeleteCriticalSection(&lock_); }

    void push(const std::string& line) {
        EnterCriticalSection(&lock_);
        lines_.push_back(line);
        LeaveCriticalSection(&lock_);
    }

    void finish() {
        EnterCriticalSection(&lock_);
        done_ = true;
        LeaveCriticalSection(&lock_);
    }

    bool pop(std::string& line) {
        bool has = false;
        EnterCriticalSection(&lock_);
        if (!lines_.empty()) {
            line = lines_.front();
            lines_.pop_front();
            has = true;
        }
        LeaveCriticalSection(&lock_);
        return has;
    }

    bool done() {
        EnterCriticalSection(&lock_);
        bool value = done_;
        LeaveCriticalSection(&lock_);
        return value;
    }

private:
    CRITICAL_SECTION lock_;
    std::deque<std::string> lines_;
    bool done_;
};

struct LineReaderContext {
    HANDLE pipe;
    LineQueue* queue;
};

DWORD WINAPI lineReaderProc(LPVOID parameter) {
    LineReaderContext* context = static_cast<LineReaderContext*>(parameter);
    std::string pending;
    char buffer[4096];
    DWORD read = 0;
    while (ReadFile(context->pipe, buffer, sizeof(buffer), &read, NULL) && read > 0) {
        pending.append(buffer, read);
        for (;;) {
            size_t newline = pending.find('\n');
            if (newline == std::string::npos) {
                break;
            }
            std::string line = pending.substr(0, newline);
            if (!line.empty() && line[line.size() - 1] == '\r') {
                line.erase(line.size() - 1);
            }
            context->queue->push(line);
            pending.erase(0, newline + 1);
        }
    }
    if (!pending.empty()) {
        context->queue->push(pending);
    }
    context->queue->finish();
    return 0;
}

void closeDownload(
    HANDLE process,
    HANDLE processThread,
    HANDLE stdoutRead,
    HANDLE stderrRead,
    HANDLE stdoutThread,
    HANDLE stderrThread) {
    if (process != NULL) {
        if (WaitForSingleObject(process, 2000) == WAIT_TIMEOUT) {
            TerminateProcess(process, 1);
        }
        WaitForSingleObject(process, INFINITE);
    }
    if (stdoutThread) {
        WaitForSingleObject(stdoutThread, INFINITE);
        CloseHandle(stdoutThread);
    }
    if (stderrThread) {
        WaitForSingleObject(stderrThread, INFINITE);
        CloseHandle(stderrThread);
    }
    if (stdoutRead) CloseHandle(stdoutRead);
    if (stderrRead) CloseHandle(stderrRead);
    if (process) CloseHandle(process);
    if (processThread) CloseHandle(processThread);
}

} // namespace

CancelToken::CancelToken() : flag_(std::make_shared<std::atomic<bool> >(false)) {}

CancelToken::CancelToken(const std::shared_ptr<std::atomic<bool> >& flag) : flag_(flag) {}

void CancelToken::cancel() const {
    if (flag_) {
        flag_->store(true, std::memory_order_seq_cst);
    }
}

bool CancelToken::isCancelled() const {
    return flag_ && flag_->load(std::memory_order_seq_cst);
}

CloseOptions CloseOptions::withNames(const std::string& names) {
    CloseOptions options;
    options.mustCloseProcessNames = names;
    options.closeTimeoutSeconds = 30;
    return options;
}

Api::Api(const std::string& path) : path_(path) {}

Api& Api::withEnv(const std::string& key, const std::string& value) {
    env_.push_back(std::make_pair(key, value));
    return *this;
}

std::string Api::defaultPath() {
    std::vector<wchar_t> module(MAX_PATH);
    DWORD length = GetModuleFileNameW(NULL, &module[0], static_cast<DWORD>(module.size()));
    if (length == 0 || length >= module.size()) {
        return std::string("..\\UpdateFolder\\aly-client.exe");
    }
    std::wstring exe(&module[0], length);
    std::wstring path = parentDir(exe) + L"\\..\\UpdateFolder\\aly-client.exe";
    return wideToUtf8(fullPath(path));
}

Response<CheckUpdateData> Api::checkUpdate(const CancelToken* cancel) const {
    RunResult run = runCollect(path_, std::vector<std::string>(1, "check_update"), env_, kShortTimeoutMs, cancel);
    return parseRun<CheckUpdateData>(run, kShortTimeoutMs, &CheckUpdateDataFromJson);
}

Response<CheckSelfUpdateData> Api::checkSelfUpdate(const CancelToken* cancel) const {
    RunResult run = runCollect(
        path_, std::vector<std::string>(1, "check_self_update"), env_, kShortTimeoutMs, cancel);
    return parseRun<CheckSelfUpdateData>(run, kShortTimeoutMs, &CheckSelfUpdateDataFromJson);
}

Response<SelfUpdateData> Api::selfUpdate(const CancelToken* cancel) const {
    RunResult run = runCollect(path_, std::vector<std::string>(1, "self_update"), env_, kShortTimeoutMs, cancel);
    return parseRun<SelfUpdateData>(run, kShortTimeoutMs, &SelfUpdateDataFromJson);
}

Response<RollbackListData> Api::listRollbackVersions() const {
    RunResult run = runCollect(
        path_, std::vector<std::string>(1, "list_rollback_versions"), env_, kShortTimeoutMs, NULL);
    return parseRun<RollbackListData>(run, kShortTimeoutMs, &RollbackListDataFromJson);
}

Response<CheckDiffData> Api::checkDiff() const {
    RunResult run = runCollect(path_, std::vector<std::string>(1, "check_diff"), env_, kShortTimeoutMs, NULL);
    return parseRun<CheckDiffData>(run, kShortTimeoutMs, &CheckDiffDataFromJson);
}

Response<CleanData> Api::clean(const CleanOptions& options) const {
    std::vector<std::string> args(1, "clean");
    if (options.all) {
        args.push_back("--all");
    } else if (options.keep > 0) {
        char value[32];
        std::snprintf(value, sizeof(value), "%d", options.keep);
        args.push_back("--keep");
        args.push_back(value);
    }
    RunResult run = runCollect(path_, args, env_, kShortTimeoutMs, NULL);
    return parseRun<CleanData>(run, kShortTimeoutMs, &CleanDataFromJson);
}

Response<RollbackData> Api::rollback(const std::string& version, const CloseOptions& options) const {
    std::vector<std::string> args;
    args.push_back("rollback");
    args.push_back("--version");
    args.push_back(version);
    appendCloseArgs(args, options);
    RunResult run = runCollect(path_, args, env_, kRollbackTimeoutMs, NULL);
    return parseRun<RollbackData>(run, kRollbackTimeoutMs, &RollbackDataFromJson);
}

Response<bool> Api::applyUpdate(const CloseOptions& options) const {
    std::vector<std::string> args(1, "apply_update");
    appendCloseArgs(args, options);
    Spawned process = spawnProcess(path_, args, env_, false);
    if (!process.error.empty()) {
        return ng<bool>(process.error);
    }
    CloseHandle(process.process);
    CloseHandle(process.thread);
    return ok<bool>(true);
}

Response<bool> Api::downloadUpdate(const DownloadOptions& options) const {
    std::vector<std::string> args(1, "download_update");
    Spawned process = spawnProcess(path_, args, env_, true);
    if (!process.error.empty()) {
        return ng<bool>(process.error);
    }

    LineQueue queue;
    LineReaderContext stdoutContext;
    stdoutContext.pipe = process.stdoutRead;
    stdoutContext.queue = &queue;
    ReaderContext stderrContext;
    stderrContext.pipe = process.stderrRead;
    HANDLE stdoutThread = CreateThread(NULL, 0, lineReaderProc, &stdoutContext, 0, NULL);
    HANDLE stderrThread = CreateThread(NULL, 0, readerProc, &stderrContext, 0, NULL);

    int stallMs = options.stallTimeoutMs > 0 ? options.stallTimeoutMs : 1800000;
    DWORD lastOutput = GetTickCount();

    for (;;) {
        if (options.cancel != NULL && options.cancel->isCancelled()) {
            closeDownload(
                process.process, process.thread, process.stdoutRead, process.stderrRead, stdoutThread, stderrThread);
            return ng<bool>("download canceled");
        }

        std::string line;
        if (queue.pop(line)) {
            lastOutput = GetTickCount();
            if (trim(line).empty()) {
                continue;
            }
            try {
                JsonValue payload = parseJson(line);
                bool success = jsonBool(payload, "isSuccess", false);
                if (!success) {
                    std::string message = jsonString(payload, "errorMsg");
                    closeDownload(
                        process.process,
                        process.thread,
                        process.stdoutRead,
                        process.stderrRead,
                        stdoutThread,
                        stderrThread);
                    return ng<bool>(message.empty() ? "download failed" : message);
                }
                if (!payload.has("data") || payload["data"].isNull()) {
                    closeDownload(
                        process.process,
                        process.thread,
                        process.stdoutRead,
                        process.stderrRead,
                        stdoutThread,
                        stderrThread);
                    return ok<bool>(true);
                }
                DownloadProgress progress;
                DownloadProgressFromJson(payload["data"], progress);
                if (progress.status == "DONE" && progress.total > 0 && options.onProgress) {
                    DownloadProgressInfo info;
                    info.file = progress.file;
                    info.ratio = static_cast<double>(progress.index) / static_cast<double>(progress.total);
                    info.status = progress.status;
                    info.index = progress.index;
                    info.total = progress.total;
                    info.downloadedBytes = progress.downloaded_bytes.value_or(0);
                    info.totalBytes = progress.total_bytes.value_or(0);
                    info.speedBps = progress.speed_bps.value_or(0);
                    try {
                        options.onProgress(info);
                    } catch (...) {
                        // 回调异常不得打断下载
                    }
                }
            } catch (const std::exception& ex) {
                closeDownload(
                    process.process,
                    process.thread,
                    process.stdoutRead,
                    process.stderrRead,
                    stdoutThread,
                    stderrThread);
                return ng<bool>(std::string("invalid progress line: ") + ex.what());
            }
            continue;
        }

        if (queue.done()) {
            closeDownload(
                process.process, process.thread, process.stdoutRead, process.stderrRead, stdoutThread, stderrThread);
            std::string err = trim(stderrContext.data);
            if (!err.empty()) {
                return ng<bool>(err);
            }
            return ng<bool>("download interrupted (aly-client exited before finish line)");
        }

        if (static_cast<int>(GetTickCount() - lastOutput) > stallMs) {
            closeDownload(
                process.process, process.thread, process.stdoutRead, process.stderrRead, stdoutThread, stderrThread);
            char message[128];
            std::snprintf(
                message,
                sizeof(message),
                "download stalled: no output for %ds, process killed",
                stallMs / 1000);
            return ng<bool>(message);
        }
        Sleep(20);
    }
}

} // namespace aly