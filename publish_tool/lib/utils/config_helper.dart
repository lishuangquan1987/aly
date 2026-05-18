import 'dart:convert';
import 'dart:io';
import 'package:path/path.dart' as path;
import 'package:publish_tool/configs/config_model.dart';

class ConfigHelper {
  ConfigHelper() {
    loadConfig();
  }

  ConfigModel? config;

  final String _configPath = path.join(
    Directory.current.path,
    "config",
    "config.json",
  );

  void saveConfig() {
    var json = config?.toJson();
    var jsonStr = jsonEncode(json);
    var f = File(_configPath);
    f.writeAsStringSync(jsonStr);
  }

  void loadConfig() {
    var file = File(_configPath);
    if (file.existsSync()) {
      var jsonStr = file.readAsStringSync();
      config = ConfigModel.fromJson(jsonDecode(jsonStr));
    } else {
      config = ConfigModel(List.empty());
    }
  }
}
