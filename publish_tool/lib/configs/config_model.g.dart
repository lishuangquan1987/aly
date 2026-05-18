// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'config_model.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

ConfigModel _$ConfigModelFromJson(Map<String, dynamic> json) => ConfigModel(
  (json['projectConfigs'] as List<dynamic>)
      .map((e) => ProjectConfig.fromJson(e as Map<String, dynamic>))
      .toList(),
);

Map<String, dynamic> _$ConfigModelToJson(ConfigModel instance) =>
    <String, dynamic>{'projectConfigs': instance.projectConfigs};

ProjectConfig _$ProjectConfigFromJson(Map<String, dynamic> json) =>
    ProjectConfig(
      (json['projectId'] as num).toInt(),
      json['projectName'] as String,
      json['projectTitle'] as String,
      json['localDir'] as String,
      json['serverUrl'] as String,
    );

Map<String, dynamic> _$ProjectConfigToJson(ProjectConfig instance) =>
    <String, dynamic>{
      'projectId': instance.projectId,
      'projectName': instance.projectName,
      'projectTitle': instance.projectTitle,
      'localDir': instance.localDir,
      'serverUrl': instance.serverUrl,
    };
