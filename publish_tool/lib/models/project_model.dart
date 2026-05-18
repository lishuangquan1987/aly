import 'package:get/get.dart';

class ProjectModel {
  var projectId = 0.obs;
  var projectName = ''.obs;
  var projectTitle = ''.obs;
  var localDir = ''.obs;
  var serverUrl=''.obs;

  var isForceUpdate = false.obs;
  var ignoreFolders = <String>[].obs;
  var ignoreFiles = <String>[].obs;
  var createAt = ''.obs;
  var isDeleted = false.obs;
  var version = ''.obs;

  var isSelected = false.obs;
}
