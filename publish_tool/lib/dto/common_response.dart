import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:json_annotation/json_annotation.dart';

part 'common_response.g.dart';

@JsonSerializable()
class CommonResponse {
  final bool isSuccess;
  final String? errorMsg;
  Object? data;

  CommonResponse({
    required this.isSuccess,
    required this.errorMsg,
    required this.data,
  });
  factory CommonResponse.fromJson(Map<String, dynamic> json) =>
      _$CommonResponseFromJson(json);

  Map<String, dynamic> toJson() => _$CommonResponseToJson(this);

  static CommonResponse ok() {
    return CommonResponse(isSuccess: true, errorMsg: null, data: null);
  }

  static CommonResponse okWithData(Object data) {
    var r = ok();
    r.data = data;
    return r;
  }

  static CommonResponse ng(Error err) {
    return ngWithMsg(err.toString());
  }

  static CommonResponse ngWithMsg(String errMsg) {
    return CommonResponse(isSuccess: false, errorMsg: errMsg, data: null);
  }
}

@JsonSerializable(genericArgumentFactories: true)
class CommonResponseWithT<T> {
  final bool isSuccess;
  final String? errorMsg;
  T? data;
  CommonResponseWithT({
    required this.data,
    required this.isSuccess,
    this.errorMsg,
  });

  static CommonResponseWithT<T> okWithData<T>(T data) {
    return CommonResponseWithT<T>(data: data, isSuccess: true, errorMsg: null);
  }

  static CommonResponseWithT<T> ng<T>(Error err) {
    return ngWithMsg(err.toString());
  }

  static CommonResponseWithT<T> ngWithMsg<T>(String errMsg) {
    return CommonResponseWithT(isSuccess: false, errorMsg: errMsg, data: null);
  }

  factory CommonResponseWithT.fromJson(
    Map<String, dynamic> json,
    T Function(Object? value) fromJson,
  ) => _$CommonResponseWithTFromJson(json, fromJson);

  Map<String, dynamic> toJson(Object? Function(T value) func) =>
      _$CommonResponseWithTToJson(this, func);
}
