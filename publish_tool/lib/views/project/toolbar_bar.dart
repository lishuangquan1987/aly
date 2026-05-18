import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:publish_tool/viewmodels/project_controller.dart';
import 'package:publish_tool/views/project/dialogs/config_editor_dialog.dart';
import 'package:publish_tool/views/project/dialogs/project_settings_dialog.dart';

class ToolbarBar extends StatelessWidget {
  final String tag;
  const ToolbarBar({super.key, required this.tag});

  @override
  Widget build(BuildContext context) {
    final ctrl = Get.find<ProjectController>(tag: tag);
    return Obx(() {
      final busy = ctrl.isBusy.value;
      return Container(
        height: 40,
        decoration: const BoxDecoration(
          color: Color(0xFF1e1e2e),
          border: Border(bottom: BorderSide(color: Color(0xFF2a2a3e))),
        ),
        padding: const EdgeInsets.symmetric(horizontal: 6),
        child: Row(
          children: [
            _btn(FluentIcons.refresh, '刷新状态', busy ? null : ctrl.refreshStatus),
            _divider(),
            _btn(FluentIcons.settings, '项目设置', () => _showSettings(context, ctrl)),
            _btn(FluentIcons.edit, '配置项', () => _showConfigEditor(context, ctrl)),
            _divider(),
            _btn(FluentIcons.build_definition, '打包', busy ? null : ctrl.buildProject),
            _btn(FluentIcons.play, '启动', ctrl.defaultLaunch),
            _btn(FluentIcons.play_resume, '自定义启动', () => _customLaunch(context, ctrl)),
            _divider(),
            _btn(FluentIcons.document, '日志', ctrl.previewLogs),
            _btn(FluentIcons.folder_open, '资源管理器', ctrl.openExplorer),
          ],
        ),
      );
    });
  }

  Widget _btn(IconData icon, String tooltip, VoidCallback? onPressed) {
    return Tooltip(
      message: tooltip,
      child: SizedBox(
        width: 36,
        height: 32,
        child: IconButton(
          icon: Icon(icon, size: 15),
          onPressed: onPressed,
        ),
      ),
    );
  }

  Widget _divider() => Container(
        width: 1,
        height: 20,
        margin: const EdgeInsets.symmetric(horizontal: 4),
        color: const Color(0xFF333344),
      );

  void _showSettings(BuildContext context, ProjectController ctrl) {
    showDialog(context: context, builder: (_) => ProjectSettingsDialog(tag: tag));
  }

  void _showConfigEditor(BuildContext context, ProjectController ctrl) {
    showDialog(context: context, builder: (_) => ConfigEditorDialog(tag: tag));
  }

  void _customLaunch(BuildContext context, ProjectController ctrl) {
    final argsCtrl = TextEditingController();
    showDialog(
      context: context,
      builder: (_) => ContentDialog(
        title: const Text('自定义启动参数'),
        content: TextBox(
          controller: argsCtrl,
          placeholder: '输入启动参数（空格分隔）',
        ),
        actions: [
          FilledButton(
            onPressed: () {
              Navigator.pop(context);
              ctrl.customLaunch(argsCtrl.text);
            },
            child: const Text('启动'),
          ),
          Button(onPressed: () => Navigator.pop(context), child: const Text('取消')),
        ],
      ),
    );
  }
}
