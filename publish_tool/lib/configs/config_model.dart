import 'package:json_annotation/json_annotation.dart';

part 'config_model.g.dart';

@JsonSerializable()
class ConfigModel {
  ConfigModel(this.projectConfigs);

  factory ConfigModel.fromJson(Map<String, dynamic> json) =>
      _$ConfigModelFromJson(json);
  Map<String, dynamic> toJson() => _$ConfigModelToJson(this);

  List<ProjectConfig> projectConfigs;
}

@JsonSerializable()
class ProjectConfig {
  int projectId;
  String projectName;
  String projectTitle;
  String localDir;
  String serverUrl;

  ProjectConfig(
    this.projectId,
    this.projectName,
    this.projectTitle,
    this.localDir,
    this.serverUrl,
  );

  factory ProjectConfig.fromJson(Map<String, dynamic> json) =>
      _$ProjectConfigFromJson(json);
  Map<String, dynamic> toJson() => _$ProjectConfigToJson(this);
}
