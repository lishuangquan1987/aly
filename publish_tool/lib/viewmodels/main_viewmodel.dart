import 'package:get/get.dart';
import 'package:publish_tool/models/project_model.dart';
import 'package:publish_tool/utils/config_helper.dart';

class MainViewmodel extends GetxController {
  var projects = <ProjectModel>[].obs;

  @override
  void onInit() {
    super.onInit();
    var configHelper = Get.find<ConfigHelper>();
    var length = configHelper.config?.projectConfigs.length ?? 0;
    for (int i = 0; i < length; i++) {
      var projectConfig = configHelper.config?.projectConfigs[i];
      var projectModel = ProjectModel();
      projectModel.projectId.value = projectConfig!.projectId;
      projectModel.projectName.value = projectConfig.projectName;
      projectModel.projectTitle.value = projectConfig.projectTitle;
      projectModel.localDir.value = projectConfig.localDir;
      projectModel.serverUrl.value = projectConfig.serverUrl;
      projects.add(projectModel);
    }
  }
}
