// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'project_change_log.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

ProjectChangeLog _$ProjectChangeLogFromJson(Map<String, dynamic> json) =>
    ProjectChangeLog(
      version: json['version'] as String? ?? '',
      logs: (json['logs'] as List<dynamic>?)
              ?.map((e) => e as String)
              .toList() ??
          [],
      time: json['time'] as String? ?? '',
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'] as String)
          : DateTime.now(),
      isDeleted: json['is_deleted'] as bool? ?? false,
    );

Map<String, dynamic> _$ProjectChangeLogToJson(ProjectChangeLog instance) =>
    <String, dynamic>{
      'version': instance.version,
      'logs': instance.logs,
      'time': instance.time,
      'created_at': instance.createdAt.toIso8601String(),
      'is_deleted': instance.isDeleted,
    };
