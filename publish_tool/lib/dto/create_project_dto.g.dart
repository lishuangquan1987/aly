// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'create_project_dto.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

CreateProjectDto _$CreateProjectDtoFromJson(Map<String, dynamic> json) =>
    CreateProjectDto(
      name: json['name'] as String,
      title: json['title'] as String,
      isForceUpdate: json['isForceUpdate'] as bool? ?? false,
      ignoreFolders: (json['ignoreFolders'] as List<dynamic>?)
              ?.map((e) => e as String)
              .toList() ??
          [],
      ignoreFiles: (json['ignoreFiles'] as List<dynamic>?)
              ?.map((e) => e as String)
              .toList() ??
          [],
    );

Map<String, dynamic> _$CreateProjectDtoToJson(CreateProjectDto instance) =>
    <String, dynamic>{
      'name': instance.name,
      'title': instance.title,
      'isForceUpdate': instance.isForceUpdate,
      'ignoreFolders': instance.ignoreFolders,
      'ignoreFiles': instance.ignoreFiles,
    };
