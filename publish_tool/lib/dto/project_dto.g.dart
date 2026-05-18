// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'project_dto.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

ProjectDto _$ProjectDtoFromJson(Map<String, dynamic> json) => ProjectDto(
      id: (json['id'] as num).toInt(),
      name: json['name'] as String? ?? '',
      title: json['title'] as String? ?? '',
      version: json['version'] as String?,
      isForceUpdate: json['force_update'] as bool? ?? false,
      ignoreFolders: (json['ignore_folders'] as List<dynamic>?)
          ?.map((e) => e as String)
          .toList(),
      ignoreFiles: (json['ignore_files'] as List<dynamic>?)
          ?.map((e) => e as String)
          .toList(),
      createAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'] as String)
          : DateTime.now(),
      isDeleted: json['is_deleted'] as bool?,
    );

Map<String, dynamic> _$ProjectDtoToJson(ProjectDto instance) =>
    <String, dynamic>{
      'id': instance.id,
      'name': instance.name,
      'title': instance.title,
      'version': instance.version,
      'force_update': instance.isForceUpdate,
      'ignore_folders': instance.ignoreFolders,
      'ignore_files': instance.ignoreFiles,
      'created_at': instance.createAt.toIso8601String(),
      'is_deleted': instance.isDeleted,
    };
