# AlyClient.JavaSDK

`aly-client` 的 Java 宿主 SDK。零第三方依赖（自带极简 JSON 解析器），兼容 **Java 8+**。

- 更新器：`UpdateFolder/aly-client.exe`（Windows）/ `UpdateFolder/aly-client`（Linux）
- 模型：`io.github.lishuangquan1987.alyclient.models` 下的类由
  `spec/aly-protocol.schema.json` 生成，**不要手改**

## 构建与安装

未发布到 Maven Central。把 `src/main/java` 拷进工程，或本地编译成 jar：

```bash
cd client/aly-client-sdk/AlyClient.JavaSDK
bash scripts/build.sh out
# 或 Windows: pwsh scripts/build.ps1 -Out out
```

把 `out/` 加入 classpath 即可。也可自行打包：

```bash
jar cf aly-client-sdk.jar -C out .
```

## 快速开始（托管生命周期）

```java
import io.github.lishuangquan1987.alyclient.*;

UpdateClient client = new UpdateClient("C:/App/UpdateFolder/aly-client.exe");
client.setOnStatusChange((status, message) -> System.out.println("[" + status + "] " + message));
client.setOnError(message -> {
    if (!message.isEmpty()) System.err.println("update error: " + message);
});
client.setConfirmDownload((version, cancelToken) -> {
    // 非强制更新时询问用户；返回即确认。阻塞时应轮询 cancelToken。
});
client.start();

// 退出前
client.stop(5000L);
```

行为与 C# / TypeScript / Go / Python SDK 一致：

- 启动时先尝试 `check_self_update` / `self_update`；
- 空闲轮询默认 300s（最小 60s，叠加 0–60s 抖动），`stop` 可中断等待；
- `force_update=true` 时不调用确认回调，直接下载并应用；
- 服务器不可达时 `has_update=false`，已下载产物保留；
- `apply_update` 以独立进程拉起后立即返回；更新器会关闭宿主并重启主程序。

## 底层 API

业务失败不抛异常，返回 `Response.isSuccess=false` + `errorMsg`：

```java
API api = new API("/path/to/aly-client");
Response<CheckUpdateData> check = api.checkUpdate();
if (check.isSuccess && check.data.hasUpdate && check.data.needDownloadUpdate) {
    Response<Void> downloaded = api.downloadUpdate(
        info -> System.out.println(info.file + " " + info.ratio),
        API.DEFAULT_DOWNLOAD_STALL_TIMEOUT_MS,
        CancelToken.NEVER);
    if (downloaded.isSuccess) {
        api.applyUpdate(new CloseOptions("YourApp", 30));
    }
}
```

| 方法 | 对应命令 | 默认超时 |
|---|---|---|
| `checkUpdate()` | `check_update` | 30s |
| `checkSelfUpdate()` | `check_self_update` | 30s |
| `selfUpdate()` | `self_update` | 30s |
| `listRollbackVersions()` | `list_rollback_versions` | 30s |
| `checkDiff()` | `check_diff` | 30s |
| `clean()` | `clean` | 30s |
| `rollback(version)` | `rollback` | 600s |
| `downloadUpdate()` | `download_update` | 无总时长；停流看门狗 1800s |
| `applyUpdate(options)` | `apply_update` | 不等待 |

下载进度是**文件计数**粒度（只统计 `DONE` 行），`DownloadProgressInfo` 同时带字节字段。

## 字段命名

生成的 Java 类用 camelCase 字段，`fromMap` 内部按协议 snake_case 读取：

```java
check.data.hasUpdate;
check.data.newVersion;
check.data.releaseNotes;
```

可选字段缺省时：`Boolean` / `Long` / `Double` 为 `null`，`List` 为 `null` 或空列表；未知字段被忽略。

## 已知边界

- Java 的 `ProcessBuilder` 没有 `CREATE_NO_WINDOW`：Windows 下从 GUI 宿主拉起控制台程序
  可能闪一个窗口。需要绝对静默时建议由原生启动器或 `aly-client-ui` 承担。
- `API.defaultPath()` 以 `user.dir` 为基准（Java 无法可靠取得宿主 exe 路径），生产环境请显式传路径。
- 不提供 `aly-client-ui` 启动器；不做自更新本地替换兜底（只委派 `self_update`）。

## 开发与测试

```bash
cd client/aly-client-sdk/AlyClient.JavaSDK
bash scripts/build.sh out
# 先构建假客户端
(cd ../../../tools/fake-aly-client && go build -o /tmp/aly-fake .)
java -cp out io.github.lishuangquan1987.alyclient.tests.TestMain /tmp/aly-fake ../../../spec/fixtures/scenarios
```

测试用仓库里的 `tools/fake-aly-client` 回放 `spec/fixtures/scenarios`，需要本机有 Go。
修改协议字段时先改 schema，再运行：

```bash
cd tools/alyprotocol
go run . -write
go run . -check
```