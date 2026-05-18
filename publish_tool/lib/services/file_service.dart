import 'dart:io';
import 'package:crypto/crypto.dart';
import 'package:path/path.dart' as p;
import 'package:publish_tool/dto/file_info_dto.dart';
import 'package:publish_tool/models/local_file_item.dart';

class FileService {
  Future<List<LocalFileItem>> scanLocalFiles(String localPath) async {
    final dir = Directory(localPath);
    if (!await dir.exists()) return [];
    final items = <LocalFileItem>[];
    await for (final entity in dir.list(recursive: true, followLinks: false)) {
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
    return items;
  }

  Future<List<FileInfoDto>> diffFiles(
      List<LocalFileItem> local, List<FileInfoDto> server) async {
    final localMd5Map = <String, String>{};
    for (final item in local) {
      final bytes = await File(item.absolutePath).readAsBytes();
      localMd5Map[item.relativePath] = md5.convert(bytes).toString();
    }
    return server.where((s) {
      final localHash = localMd5Map[s.fileRelativePath];
      return localHash == null || localHash != s.md5;
    }).toList();
  }
}
