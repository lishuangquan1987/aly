import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:publish_tool/api/base_api.dart';
import 'package:publish_tool/dto/common_response.dart';
import 'package:publish_tool/dto/file_info_dto.dart';

class FileApi extends BaseApi {
  FileApi(super.baseUrl);

  Future<CommonResponse> uploadFile(
    String absoluteFilePath,
    String relativeFilePath,
    String projectName, {
    Function(int, int)? progress,
    CancelToken? token,
  }) {
    var url = "api/file/upload_file";
    return doUploadFile(
      url,
      absoluteFilePath,
      {"projectName": projectName, "relativeFileName": relativeFilePath},
      progress,
      token,
    );
  }

  Future<CommonResponseWithT<List<FileInfoDto>>> getAllFilesByProjectId(
    int projectId,
  ) {
    var url = "api/file/get_all_files/$projectId";
    return doGet(
      url,
      (o) => (o as List)
          .map((e) => FileInfoDto.fromJson(e as Map<String, dynamic>))
          .toList(),
    );
  }

  Future<CommonResponse> downloadFile(
    String serverFilePath,
    String savePath, {
    Function(int, int)? progress,
    CancelToken? token,
  }) async {
    var url = "api/file/download_file?path=$serverFilePath";
    return await doDownloadFile(url, savePath, progress, token);
  }
}
