import 'package:dio/dio.dart';
import 'package:publish_tool/dto/common_response.dart';
import 'package:publish_tool/logger/log_helper.dart';

class BaseApi {
  late Dio _dio;
  String baseUrl = "";
  BaseApi(this.baseUrl) {
    if (!baseUrl.endsWith('/')) baseUrl += '/';
    _dio = Dio(
      BaseOptions(
        baseUrl: baseUrl,
        connectTimeout: const Duration(seconds: 5),
        receiveTimeout: const Duration(seconds: 30),
      ),
    );
    _dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) {
          LogHelper.trace(options);
          return handler.next(options);
        },
        onResponse: (response, handler) {
          LogHelper.trace(response);
          return handler.next(response);
        },
        onError: (error, handler) {
          LogHelper.errorWithError(error);
          return handler.next(error);
        },
      ),
    );
  }

  Map<String, dynamic> _toMap(dynamic data) {
    if (data is Map<String, dynamic>) return data;
    throw Exception('服务端返回非JSON数据: $data');
  }

  /// 将 dto 转为 dio 可识别的 Map，dto 需实现 toJson()
  dynamic _toBody(Object data) {
    if (data is Map<String, dynamic> || data is String || data is FormData) {
      return data;
    }
    // 调用 dto 的 toJson()
    return (data as dynamic).toJson() as Map<String, dynamic>;
  }

  Future<CommonResponse> doPost(String url, Object data) async {
    try {
      final response = await _dio.post(url, data: _toBody(data));
      return CommonResponse.fromJson(_toMap(response.data));
    } catch (e) {
      return CommonResponse.ngWithMsg(e.toString());
    }
  }

  Future<CommonResponseWithT<T>> doPostWithT<T>(
    String url,
    Object data,
    T Function(Object? value) fromJson,
  ) async {
    try {
      final response = await _dio.post(url, data: _toBody(data));
      return CommonResponseWithT.fromJson(_toMap(response.data), fromJson);
    } catch (e) {
      return CommonResponseWithT.ngWithMsg(e.toString());
    }
  }

  Future<CommonResponseWithT<T>> doGet<T>(
    String url,
    T Function(Object? value) fromJson,
  ) async {
    try {
      final response = await _dio.get(url);
      return CommonResponseWithT.fromJson(_toMap(response.data), fromJson);
    } catch (e) {
      return CommonResponseWithT.ngWithMsg(e.toString());
    }
  }

  Future<CommonResponse> doUploadFile(
    String url,
    String filePath,
    Map<String, String>? data,
    Function(int send, int total)? progress,
    CancelToken? token,
  ) async {
    try {
      FormData formData = FormData.fromMap({
        "file": await MultipartFile.fromFile(filePath),
      });
      if (data != null) {
        data.forEach((key, value) {
          formData.fields.add(MapEntry(key, value));
        });
      }
      final response = await _dio.post(
        url,
        data: formData,
        onSendProgress: progress,
        cancelToken: token,
      );
      return CommonResponse.fromJson(_toMap(response.data));
    } catch (e) {
      return CommonResponse.ngWithMsg(e.toString());
    }
  }

  Future<CommonResponse> doDownloadFile(
    String url,
    String savePath,
    Function(int, int)? progress,
    CancelToken? token,
  ) async {
    try {
      await _dio.download(
        url,
        savePath,
        onReceiveProgress: progress,
        cancelToken: token,
      );
      return CommonResponse.ok();
    } catch (e) {
      return CommonResponse.ngWithMsg(e.toString());
    }
  }
}
