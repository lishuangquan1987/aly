import 'package:json_annotation/json_annotation.dart';

part 'server_os_info_dto.g.dart';

@JsonSerializable()
class ServerOsInfoDto {
  String os;
  String platform;
  String goARCH;
  String version;
  int numCPU;
  String cpuName;
  double cpuMhz;
  double diskUsed;
  double diskFree;
  double diskTotal;
  double diskUsedPercent;

  ServerOsInfoDto(
    this.os,
    this.platform,
    this.goARCH,
    this.version,
    this.numCPU,
    this.cpuName,
    this.cpuMhz,
    this.diskUsed,
    this.diskFree,
    this.diskTotal,
    this.diskUsedPercent,
  );

  factory ServerOsInfoDto.fromJson(Map<String, dynamic> json) =>
      _$ServerOsInfoDtoFromJson(json);
  Map<String, dynamic> toJson() => _$ServerOsInfoDtoToJson(this);
}
