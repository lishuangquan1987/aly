import 'package:get/get.dart';
import 'package:publish_tool/api/project_api.dart';
import 'package:publish_tool/dto/common_response.dart';
import 'package:publish_tool/dto/create_project_dto.dart';
import 'package:publish_tool/dto/project_dto.dart';
import 'package:publish_tool/models/project_config.dart';
import 'package:publish_tool/services/config_service.dart';

class AppController extends GetxController {
  final _configService = Get.find<ConfigService>();

  final projectConfigs = <ProjectConfig>[].obs;
  final filterKeyword = ''.obs;
  final openTabs = <ProjectConfig>[].obs;
  final activeTabIndex = 0.obs;

  List<ProjectConfig> get filteredProjects => projectConfigs
      .where((p) =>
          filterKeyword.value.isEmpty ||
          p.title.contains(filterKeyword.value) ||
          p.name.contains(filterKeyword.value))
      .toList();

  @override
  void onInit() {
    super.onInit();
    loadConfig();
  }

  Future<void> loadConfig() async {
    final configs = await _configService.loadConfigs();
    configs.sort((a, b) => a.sortOrder.compareTo(b.sortOrder));
    projectConfigs.assignAll(configs);
  }

  Future<void> saveConfig() async {
    await _configService.saveConfigs(projectConfigs);
  }

  /// 服务端项目：只在服务端创建，不保存本地配置
  Future<CommonResponseWithT<ProjectDto>> createServerProject(
          String serverUrl, CreateProjectDto dto) =>
      ProjectApi(serverUrl).createProject(dto);

  /// 客户端项目：不调用服务端，直接保存本地配置
  Future<void> addLocalProject(ProjectConfig config) async {
    config.sortOrder = projectConfigs.length;
    projectConfigs.add(config);
    await saveConfig();
  }

  Future<void> deleteProject(int index) async {
    if (index < 0 || index >= projectConfigs.length) return;
    final config = projectConfigs[index];
    final tabIdx = openTabs.indexWhere((t) => t.name == config.name);
    if (tabIdx >= 0) closeTab(tabIdx);
    projectConfigs.removeAt(index);
    _reorderSortOrder();
    await saveConfig();
  }

  void moveUp(int index) {
    if (index <= 0) return;
    final tmp = projectConfigs[index];
    projectConfigs[index] = projectConfigs[index - 1];
    projectConfigs[index - 1] = tmp;
    _reorderSortOrder();
    saveConfig();
  }

  void moveDown(int index) {
    if (index >= projectConfigs.length - 1) return;
    final tmp = projectConfigs[index];
    projectConfigs[index] = projectConfigs[index + 1];
    projectConfigs[index + 1] = tmp;
    _reorderSortOrder();
    saveConfig();
  }

  void _reorderSortOrder() {
    for (int i = 0; i < projectConfigs.length; i++) {
      projectConfigs[i].sortOrder = i;
    }
    projectConfigs.refresh();
  }

  void openTab(ProjectConfig config) {
    final idx = openTabs.indexWhere((t) => t.name == config.name);
    if (idx >= 0) {
      activeTabIndex.value = idx;
    } else {
      openTabs.add(config);
      activeTabIndex.value = openTabs.length - 1;
    }
  }

  void closeTab(int index) {
    if (index < 0 || index >= openTabs.length) return;
    openTabs.removeAt(index);
    if (activeTabIndex.value >= openTabs.length) {
      activeTabIndex.value = openTabs.length - 1;
    }
  }

  Future<void> refreshAllProjects() async {
    await loadConfig();
  }
}
