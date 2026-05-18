import 'dart:io';
import 'package:crypto/crypto.dart';
import 'package:dio/dio.dart';
import 'package:get/get.dart';
import 'package:intl/intl.dart';
import 'package:path/path.dart' as p;
import 'package:publish_tool/api/file_api.dart';
import 'package:publish_tool/api/project_api.dart';
import 'package:publish_tool/dto/file_info_dto.dart';
import 'package:publish_tool/dto/project_change_log.dart';
import 'package:publish_tool/dto/server_os_info_dto.dart';
import 'package:publish_tool/dto/update_project_dto.dart';
import 'package:publish_tool/models/local_file_item.dart';
import 'package:publish_tool/models/project_config.dart';
import 'package:publish_tool/models/upload_file_item.dart';
import 'package:publish_tool/services/process_service.dart';

class ProjectController extends GetxController {
  final ProjectConfig projectConfig;
  ProjectController(this.projectConfig);

  ProjectApi get _projectApi => ProjectApi(projectConfig.serverUrl);
  FileApi get _fileApi => FileApi(projectConfig.serverUrl);
  final _processService = Get.find<ProcessService>();

  CancelToken? _cancelToken;

  final serverOsInfo = Rxn<ServerOsInfoDto>();
  final serverVersion = ''.obs;
  final serverChangeLogs = <ProjectChangeLog>[].obs;
  final localFiles = <LocalFileItem>[].obs;
  final localFileFilter = ''.obs;
  final uploadQueue = <UploadFileItem>[].obs;
  final newVersion = ''.obs;
  final newChangeLogs = ''.obs;
  final appendToLatest = false.obs;
  final autoRefreshAfterPush = true.obs;
  final statusMessage = ''.obs;
  final isBusy = false.obs;

  List<LocalFileItem> get filteredLocalFiles => localFiles
      .where((f) =>
          localFileFilter.value.isEmpty ||
          f.fileName.contains(localFileFilter.value))
      .toList();

  @override
  void onInit() {
    super.onInit();
    refreshStatus();
  }

  Future<void> refreshStatus() async {
    isBusy.value = true;
    statusMessage.value = '正在刷新服务器信息...';

    final osRes = await _projectApi.getProjectOsInfo(projectConfig.serverId);
    if (osRes.isSuccess) serverOsInfo.value = osRes.data;

    final logsRes =
        await _projectApi.getProjectChangeLogs(projectConfig.serverId);
    if (logsRes.isSuccess) {
      serverChangeLogs.assignAll(logsRes.data ?? []);
      serverVersion.value =
          logsRes.data?.isNotEmpty == true ? logsRes.data!.first.version : '';
      statusMessage.value = '刷新完成';
    } else {
      statusMessage.value = '刷新失败: ${logsRes.errorMsg}';
    }

    isBusy.value = false;
  }

  Future<void> loadLocalFiles() async {
    if (projectConfig.localPath.isEmpty) {
      statusMessage.value = '本地路径未配置';
      return;
    }
    isBusy.value = true;
    statusMessage.value = '正在扫描本地文件...';
    final res = await _scanLocalFiles(projectConfig.localPath);
    if (res.isSuccess) {
      localFiles.assignAll(res.data ?? []);
      statusMessage.value = '扫描完成，共 ${res.data?.length ?? 0} 个文件';
    } else {
      statusMessage.value = '扫描失败: ${res.errorMsg}';
    }
    isBusy.value = false;
  }

  Future<void> openLocalFolder() async {
    if (projectConfig.localPath.isNotEmpty) {
      await _processService.openFolder(projectConfig.localPath);
    }
  }

  void addToUploadQueue(List<LocalFileItem> items) {
    for (final item in items) {
      if (!uploadQueue.any((u) => u.relativePath == item.relativePath)) {
        uploadQueue.add(UploadFileItem(
          fileName: item.fileName,
          localPath: item.absolutePath,
          relativePath: item.relativePath,
          lastModified: item.lastModified,
        ));
      }
    }
  }

  void removeFromUploadQueue(UploadFileItem item) {
    uploadQueue.remove(item);
  }

  Future<void> pushAll() async {
    final checked = localFiles.where((f) => f.isChecked).toList();
    if (checked.isEmpty) {
      statusMessage.value = '请先选择要推送的文件';
      return;
    }
    addToUploadQueue(checked);
    await _uploadQueue();
  }

  Future<void> _uploadQueue() async {
    if (uploadQueue.isEmpty) return;
    _cancelToken = CancelToken();
    isBusy.value = true;
    int done = 0;
    for (final item in uploadQueue) {
      if (_cancelToken!.isCancelled) break;
      item.status = UploadStatus.uploading;
      uploadQueue.refresh();
      final res = await _fileApi.uploadFile(
        item.localPath,
        item.relativePath,
        projectConfig.name,
        token: _cancelToken,
      );
      if (res.isSuccess) {
        item.status = UploadStatus.done;
        done++;
      } else {
        item.status = UploadStatus.failed;
        statusMessage.value = '上传失败: ${res.errorMsg}';
      }
      uploadQueue.refresh();
    }
    isBusy.value = false;
    statusMessage.value = '上传完成 $done/${uploadQueue.length}';
    if (autoRefreshAfterPush.value) await refreshStatus();
  }

  void stop() {
    _cancelToken?.cancel('用户停止');
    isBusy.value = false;
    statusMessage.value = '已停止';
  }

  Future<void> downloadAll() async {
    if (projectConfig.localPath.isEmpty) {
      statusMessage.value = '本地路径未配置';
      return;
    }
    isBusy.value = true;
    _cancelToken = CancelToken();
    statusMessage.value = '正在下载所有文件...';

    final serverRes =
        await _fileApi.getAllFilesByProjectId(projectConfig.serverId);
    if (!serverRes.isSuccess) {
      statusMessage.value = '获取服务端文件失败: ${serverRes.errorMsg}';
      isBusy.value = false;
      return;
    }
    final serverFiles = serverRes.data ?? [];
    for (final f in serverFiles) {
      if (_cancelToken!.isCancelled) break;
      final res = await _downloadFile(f, projectConfig.localPath,
          token: _cancelToken);
      if (!res.isSuccess) {
        statusMessage.value = '下载失败: ${res.errorMsg}';
        isBusy.value = false;
        return;
      }
    }
    statusMessage.value = '下载完成，共 ${serverFiles.length} 个文件';
    isBusy.value = false;
  }

  Future<void> pullAll() async {
    if (projectConfig.localPath.isEmpty) {
      statusMessage.value = '本地路径未配置';
      return;
    }
    isBusy.value = true;
    _cancelToken = CancelToken();
    statusMessage.value = '正在对比文件...';

    final serverRes =
        await _fileApi.getAllFilesByProjectId(projectConfig.serverId);
    if (!serverRes.isSuccess) {
      statusMessage.value = '获取服务端文件失败: ${serverRes.errorMsg}';
      isBusy.value = false;
      return;
    }
    final localRes = await _scanLocalFiles(projectConfig.localPath);
    if (!localRes.isSuccess) {
      statusMessage.value = '扫描本地文件失败: ${localRes.errorMsg}';
      isBusy.value = false;
      return;
    }
    final diffRes =
        await _diffFiles(localRes.data ?? [], serverRes.data ?? []);
    if (!diffRes.isSuccess) {
      statusMessage.value = '对比失败: ${diffRes.errorMsg}';
      isBusy.value = false;
      return;
    }
    final diff = diffRes.data ?? [];
    statusMessage.value = '需要下载 ${diff.length} 个文件';
    for (final f in diff) {
      if (_cancelToken!.isCancelled) break;
      final res = await _downloadFile(f, projectConfig.localPath,
          token: _cancelToken);
      if (!res.isSuccess) {
        statusMessage.value = '下载失败: ${res.errorMsg}';
        isBusy.value = false;
        return;
      }
    }
    statusMessage.value = '拉取完成，更新 ${diff.length} 个文件';
    isBusy.value = false;
  }

  Future<void> refreshFiles() async {
    await loadLocalFiles();
  }

  void autoGenerateVersion() {
    newVersion.value = DateFormat('yyyyMMdd-HHmm').format(DateTime.now());
  }

  Future<void> pushUpdate() async {
    if (newVersion.value.isEmpty) {
      statusMessage.value = '请输入版本号';
      return;
    }
    await _uploadQueue();
    statusMessage.value = '推送更新完成，版本: ${newVersion.value}';
  }

  Future<void> openProjectSettings() async {}

  Future<void> openConfigEditor() async {}

  Future<void> buildProject() async {
    statusMessage.value = '正在打包...';
    isBusy.value = true;
    try {
      final result = await _processService.buildProject(
          'flutter build windows --release', projectConfig.localPath);
      statusMessage.value =
          result.exitCode == 0 ? '打包成功' : '打包失败: ${result.stderr}';
    } catch (e) {
      statusMessage.value = '打包失败: $e';
    } finally {
      isBusy.value = false;
    }
  }

  Future<void> defaultLaunch() async {
    if (projectConfig.exePath.isEmpty) {
      statusMessage.value = 'exe 路径未配置';
      return;
    }
    await _processService.launchExe(projectConfig.exePath);
  }

  Future<void> customLaunch(String args) async {
    if (projectConfig.exePath.isEmpty) {
      statusMessage.value = 'exe 路径未配置';
      return;
    }
    await _processService.launchExe(projectConfig.exePath,
        args: args.split(' ').where((s) => s.isNotEmpty).toList());
  }

  Future<void> previewLogs() async {}

  Future<void> openExplorer() async {
    await openLocalFolder();
  }

  Future<void> updateProjectSettings(
      String title, String exePath, String localPath) async {
    projectConfig.title = title;
    projectConfig.exePath = exePath;
    projectConfig.localPath = localPath;
    final dto = UpdateProjectDto(
      id: projectConfig.serverId,
      title: title,
      isForceUpdate: false,
      ignoreFolders: [],
      ignoreFiles: [],
    );
    final res = await _projectApi.updateProject(dto);
    statusMessage.value = res.isSuccess ? '设置已保存' : '保存失败: ${res.errorMsg}';
  }

  // ── 本地文件工具方法 ──────────────────────────────────────────

  Future<_LocalResult<List<LocalFileItem>>> _scanLocalFiles(
      String localPath) async {
    try {
      final dir = Directory(localPath);
      if (!await dir.exists()) return _LocalResult.ok([]);
      final items = <LocalFileItem>[];
      await for (final entity
          in dir.list(recursive: true, followLinks: false)) {
        if (entity is File) {
          final stat = await entity.stat();
          final relativePath =
              p.relative(entity.path, from: localPath).replaceAll('\\', '/');
          items.add(LocalFileItem(
            fileName: p.basename(entity.path),
            absolutePath: entity.path,
            relativePath: relativePath,
            lastModified: stat.modified,
          ));
        }
      }
      return _LocalResult.ok(items);
    } catch (e) {
      return _LocalResult.err(e.toString());
    }
  }

  Future<_LocalResult<List<FileInfoDto>>> _diffFiles(
      List<LocalFileItem> local, List<FileInfoDto> server) async {
    try {
      final localMd5Map = <String, String>{};
      for (final item in local) {
        final bytes = await File(item.absolutePath).readAsBytes();
        localMd5Map[item.relativePath] = md5.convert(bytes).toString();
      }
      final diff = server.where((s) {
        final localHash = localMd5Map[s.fileRelativePath];
        return localHash == null || localHash != s.md5;
      }).toList();
      return _LocalResult.ok(diff);
    } catch (e) {
      return _LocalResult.err(e.toString());
    }
  }

  Future<_LocalResult<void>> _downloadFile(
    FileInfoDto serverFile,
    String localBasePath, {
    Function(int, int)? progress,
    CancelToken? token,
  }) async {
    try {
      final savePath = p.join(localBasePath,
          serverFile.fileRelativePath.replaceAll('/', p.separator));
      await File(savePath).parent.create(recursive: true);
      final res = await _fileApi.downloadFile(
        serverFile.fileAbsolutePath,
        savePath,
        progress: progress,
        token: token,
      );
      return res.isSuccess
          ? _LocalResult.ok(null)
          : _LocalResult.err(res.errorMsg ?? '');
    } catch (e) {
      return _LocalResult.err(e.toString());
    }
  }
}

class _LocalResult<T> {
  final bool isSuccess;
  final T? data;
  final String? errorMsg;
  _LocalResult._(this.isSuccess, this.data, this.errorMsg);
  factory _LocalResult.ok(T data) => _LocalResult._(true, data, null);
  factory _LocalResult.err(String msg) => _LocalResult._(false, null, msg);
}
