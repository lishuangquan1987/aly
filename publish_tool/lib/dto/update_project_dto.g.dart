// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'update_project_dto.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

UpdateProjectDto _$UpdateProjectDtoFromJson(Map<String, dynamic> json) =>
    UpdateProjectDto(
      id: (json['id'] as num).toInt(),
      title: json['title'] as String,
      isForceUpdate: json['isForceUpdate'] as bool? ?? false,
      ignoreFiles: (json['ignoreFiles'] as List<dynamic>?)
              ?.map((e) => e as String)
              .toList() ??
          [],
      ignoreFolders: (json['ignoreFolders'] as List<dynamic>?)
              ?.map((e) => e as String)
              .toList() ??
          [],
    );

Map<String, dynamic> _$UpdateProjectDtoToJson(UpdateProjectDto instance) =>
    <String, dynamic>{
      'id': instance.id,
      'title': instance.title,
      'isForceUpdate': instance.isForceUpdate,
      'ignoreFolders': instance.ignoreFolders,
      'ignoreFiles': instance.ignoreFiles,
    };
