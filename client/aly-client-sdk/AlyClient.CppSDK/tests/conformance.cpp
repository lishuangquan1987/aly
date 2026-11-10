#include "aly/api.h"
#include "aly/client.h"

#include <chrono>
#include <cstdio>
#include <functional>
#include <iostream>
#include <mutex>
#include <string>
#include <vector>

namespace {

std::string g_fake;
std::string g_scenarios;
int g_pass = 0;
int g_fail = 0;

typedef std::function<std::string()> CaseBody;

aly::Api makeApi(const std::string& scenario) {
    aly::Api api(g_fake);
    api.withEnv("ALY_FAKE_SCENARIO", g_scenarios + "\\" + scenario + ".json");
    return api;
}

void waitForStatus(aly::UpdateClient& client, const std::string& want, int timeoutSeconds) {
    std::chrono::steady_clock::time_point deadline =
        std::chrono::steady_clock::now() + std::chrono::seconds(timeoutSeconds);
    while (std::chrono::steady_clock::now() < deadline) {
        if (client.status() == want) {
            return;
        }
        Sleep(20);
    }
    throw std::runtime_error(
        "waitForStatus timeout, last=" + client.status() + ", error=" + client.lastError());
}

void check(const std::string& name, CaseBody body) {
    std::string error;
    try {
        error = body();
    } catch (const std::exception& ex) {
        error = ex.what();
    } catch (...) {
        error = "unknown exception";
    }
    if (error.empty()) {
        ++g_pass;
        std::cout << "PASS: " << name << std::endl;
    } else {
        ++g_fail;
        std::cout << "FAIL: " << name << " - " << error << std::endl;
    }
}

} // namespace

int main(int argc, char** argv) {
    if (argc < 3) {
        std::cerr << "usage: conformance <fake-aly-client.exe> <scenariosDir>" << std::endl;
        return 2;
    }
    g_fake = argv[1];
    g_scenarios = argv[2];
    SetConsoleOutputCP(CP_UTF8);

    check("check_update available", []() -> std::string {
        aly::Response<aly::CheckUpdateData> response = makeApi("update_available").checkUpdate();
        if (!response.isSuccess) {
            return response.errorMsg;
        }
        if (!response.data.has_value()) {
            return "no data";
        }
        const aly::CheckUpdateData& data = response.data.get();
        if (!data.has_update || !data.need_download_update) {
            return "has_update/need_download_update false";
        }
        if (data.new_version.value_or("") != "1.0.1") {
            return "new_version mismatch";
        }
        if (!data.force_update.has_value() || data.force_update.get()) {
            return "force_update should be false";
        }
        if (!data.release_notes.has_value() || data.release_notes.get().size() != 2) {
            return "release_notes mismatch";
        }
        return std::string();
    });

    check("check_update latest/offline", []() -> std::string {
        const char* scenarios[] = {"already_latest", "offline"};
        for (int i = 0; i < 2; ++i) {
            aly::Response<aly::CheckUpdateData> response = makeApi(scenarios[i]).checkUpdate();
            if (!response.isSuccess) {
                return std::string(scenarios[i]) + ": " + response.errorMsg;
            }
            if (!response.data.has_value() || response.data.get().has_update) {
                return std::string(scenarios[i]) + ": has_update true";
            }
        }
        return std::string();
    });

    check("download_update progress", []() -> std::string {
        std::vector<aly::DownloadProgressInfo> progress;
        aly::DownloadOptions options;
        options.onProgress = [&progress](const aly::DownloadProgressInfo& info) {
            progress.push_back(info);
        };
        aly::Response<bool> response = makeApi("update_available").downloadUpdate(options);
        if (!response.isSuccess) {
            return response.errorMsg;
        }
        if (progress.size() != 1) {
            return "progress count mismatch";
        }
        if (progress[0].file != "app.exe" || progress[0].ratio != 1.0
            || progress[0].status != "DONE") {
            return "progress info mismatch";
        }
        if (progress[0].downloadedBytes != 1000) {
            return "downloadedBytes mismatch";
        }
        return std::string();
    });

    check("download_update partial failure", []() -> std::string {
        aly::Response<bool> response = makeApi("download_partial_failure").downloadUpdate();
        if (response.isSuccess) {
            return "expected failure";
        }
        if (response.errorMsg.find("checksum mismatch") == std::string::npos) {
            return "errorMsg=" + response.errorMsg;
        }
        return std::string();
    });

    check("download_update stall watchdog", []() -> std::string {
        aly::DownloadOptions options;
        options.stallTimeoutMs = 1000;
        aly::Response<bool> response = makeApi("download_stall").downloadUpdate(options);
        if (response.isSuccess) {
            return "expected failure";
        }
        if (response.errorMsg.find("stalled") == std::string::npos) {
            return "errorMsg=" + response.errorMsg;
        }
        return std::string();
    });

    check("apply_update returns immediately", []() -> std::string {
        std::chrono::steady_clock::time_point started = std::chrono::steady_clock::now();
        aly::Response<bool> response =
            makeApi("update_available").applyUpdate(aly::CloseOptions::withNames("A,B"));
        long long elapsed = std::chrono::duration_cast<std::chrono::milliseconds>(
                                std::chrono::steady_clock::now() - started)
                                .count();
        if (!response.isSuccess) {
            return response.errorMsg;
        }
        if (elapsed > 10000) {
            return "apply took too long";
        }
        return std::string();
    });

    check("self_update flow", []() -> std::string {
        aly::Api api = makeApi("self_update");
        aly::Response<aly::CheckSelfUpdateData> check = api.checkSelfUpdate();
        if (!check.isSuccess || !check.data.has_value() || !check.data.get().need_update) {
            return "check_self_update mismatch";
        }
        aly::Response<aly::SelfUpdateData> updated = api.selfUpdate();
        if (!updated.isSuccess || !updated.data.has_value()) {
            return "self_update failed: " + updated.errorMsg;
        }
        if (!updated.data.get().updated) {
            return "updated false";
        }
        if (updated.data.get().old_path.value_or("") != "aly-client.exe.old") {
            return "old_path mismatch";
        }
        return std::string();
    });

    check("rollback flow", []() -> std::string {
        aly::Api api = makeApi("rollback");
        aly::Response<aly::RollbackListData> list = api.listRollbackVersions();
        if (!list.isSuccess || !list.data.has_value() || list.data.get().versions.size() != 2) {
            return "versions mismatch";
        }
        aly::Response<aly::RollbackData> rolled = api.rollback("1.0.0");
        if (!rolled.isSuccess || !rolled.data.has_value()
            || rolled.data.get().version != "1.0.0") {
            return "rollback failed: " + rolled.errorMsg;
        }
        return std::string();
    });

    check("check_diff and clean", []() -> std::string {
        aly::Response<aly::CheckDiffData> diff = makeApi("check_diff").checkDiff();
        if (!diff.isSuccess || !diff.data.has_value() || diff.data.get().files.size() != 1
            || diff.data.get().files[0].path != "app.exe") {
            return "check_diff mismatch";
        }
        aly::CleanOptions clean;
        clean.all = true;
        aly::Response<aly::CleanData> cleaned = makeApi("clean_all").clean(clean);
        if (!cleaned.isSuccess || !cleaned.data.has_value()
            || cleaned.data.get().mode != "all") {
            return "clean mismatch";
        }
        return std::string();
    });

    check("missing executable returns failure", []() -> std::string {
        aly::Api api("C:\\aly-cpp-missing\\aly-client.exe");
        aly::Response<aly::CheckUpdateData> response = api.checkUpdate();
        if (response.isSuccess) {
            return "expected failure";
        }
        if (response.errorMsg.empty()) {
            return "empty errorMsg";
        }
        return std::string();
    });

    check("defaultPath points to UpdateFolder", []() -> std::string {
        std::string path = aly::Api::defaultPath();
        if (path.find("UpdateFolder") == std::string::npos) {
            return path;
        }
        return std::string();
    });

    check("managed force update full chain", []() -> std::string {
        std::mutex lock;
        std::vector<std::string> statuses;
        aly::UpdateClient client(makeApi("force_update"));
        client.setOnStatus([&lock, &statuses](const std::string& status, const std::string&) {
            std::lock_guard<std::mutex> guard(lock);
            statuses.push_back(status);
        });
        client.start();
        waitForStatus(client, aly::Status::APPLY_UPDATE, 10);
        if (!client.stop(10000)) {
            return "stop timeout";
        }
        std::lock_guard<std::mutex> guard(lock);
        if (statuses.empty() || statuses[0] != aly::Status::DISCOVERED_UPDATE) {
            return "status sequence mismatch";
        }
        bool downloading = false;
        bool downloaded = false;
        for (size_t i = 0; i < statuses.size(); ++i) {
            downloading = downloading || statuses[i] == aly::Status::DOWNLOADING_UPDATE;
            downloaded = downloaded || statuses[i] == aly::Status::DOWNLOADED_UPDATE;
        }
        if (!downloading || !downloaded || statuses.back() != aly::Status::APPLY_UPDATE) {
            return "status sequence mismatch";
        }
        if (!client.lastError().empty()) {
            return "lastError=" + client.lastError();
        }
        return std::string();
    });

    check("managed non-force waits for confirmations", []() -> std::string {
        std::mutex lock;
        std::vector<std::string> downloads;
        std::vector<std::string> applies;
        aly::UpdateClient client(makeApi("update_available"));
        client.setConfirmDownload([&lock, &downloads](const std::string& version, const aly::CancelToken&) {
            std::lock_guard<std::mutex> guard(lock);
            downloads.push_back(version);
        });
        client.setConfirmApply([&lock, &applies](const std::string& version, const aly::CancelToken&) {
            std::lock_guard<std::mutex> guard(lock);
            applies.push_back(version);
        });
        client.start();
        waitForStatus(client, aly::Status::APPLY_UPDATE, 10);
        if (!client.stop(10000)) {
            return "stop timeout";
        }
        std::lock_guard<std::mutex> guard(lock);
        if (downloads.size() != 1 || downloads[0] != "1.0.1") {
            return "download confirmations mismatch";
        }
        if (applies.size() != 1 || applies[0] != "1.0.1") {
            return "apply confirmations mismatch";
        }
        return std::string();
    });

    check("managed offline stays idle", []() -> std::string {
        aly::UpdateClient client(makeApi("offline"));
        std::mutex lock;
        std::vector<std::string> asked;
        client.setConfirmDownload([&lock, &asked](const std::string& version, const aly::CancelToken&) {
            std::lock_guard<std::mutex> guard(lock);
            asked.push_back(version);
        });
        client.start();
        Sleep(500);
        if (client.status() != aly::Status::NONE) {
            return "status=" + client.status();
        }
        client.stop(10000);
        std::lock_guard<std::mutex> guard(lock);
        if (!asked.empty()) {
            return "confirm should not be called";
        }
        return std::string();
    });

    check("stop interrupts pending confirmation", []() -> std::string {
        aly::UpdateClient client(makeApi("update_available"));
        client.setConfirmDownload([](const std::string&, const aly::CancelToken& token) {
            while (!token.isCancelled()) {
                Sleep(10);
            }
        });
        client.start();
        waitForStatus(client, aly::Status::DISCOVERED_UPDATE, 10);
        std::chrono::steady_clock::time_point started = std::chrono::steady_clock::now();
        bool stopped = client.stop(10000);
        long long elapsed = std::chrono::duration_cast<std::chrono::milliseconds>(
                                std::chrono::steady_clock::now() - started)
                                .count();
        if (!stopped) {
            return "stop timeout";
        }
        if (elapsed > 3000) {
            return "stop too slow";
        }
        return std::string();
    });

    check("stop is idempotent", []() -> std::string {
        aly::UpdateClient client(makeApi("offline"));
        client.start();
        Sleep(100);
        if (!client.stop(10000) || !client.stop(10000)) {
            return "stop returned false";
        }
        if (client.isRunning()) {
            return "isRunning true";
        }
        return std::string();
    });

    check("cancel token behavior", []() -> std::string {
        aly::CancelToken token;
        if (token.isCancelled()) {
            return "new token cancelled";
        }
        token.cancel();
        if (!token.isCancelled()) {
            return "cancel ignored";
        }
        return std::string();
    });

    std::cout << "SUMMARY: PASS=" << g_pass << " FAIL=" << g_fail << std::endl;
    return g_fail == 0 ? 0 : 1;
}