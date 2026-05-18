import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:publish_tool/viewmodels/project_controller.dart';

class OperationButtons extends StatelessWidget {
  final String tag;
  const OperationButtons({super.key, required this.tag});

  @override
  Widget build(BuildContext context) {
    final ctrl = Get.find<ProjectController>(tag: tag);
    return Obx(() {
      final busy = ctrl.isBusy.value;
      return Container(
        width: 48,
        decoration: const BoxDecoration(
          border: Border(
            left: BorderSide(color: Color(0xFF2a2a3e)),
            right: BorderSide(color: Color(0xFF2a2a3e)),
          ),
        ),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            _btn(FluentIcons.upload, '全部推送', busy ? null : ctrl.pushAll,
                color: const Color(0xFF0078d4)),
            _btn(FluentIcons.stop, '停止', busy ? ctrl.stop : null,
                color: const Color(0xFFaa3333)),
            _btn(FluentIcons.download, '全部下载', busy ? null : ctrl.downloadAll,
                color: const Color(0xFF2d7a2d)),
            _btn(FluentIcons.cloud_download, '拉取更新', busy ? null : ctrl.pullAll),
            _btn(FluentIcons.refresh, '刷新文件', busy ? null : ctrl.refreshFiles),
          ],
        ),
      );
    });
  }

  Widget _btn(IconData icon, String tooltip, VoidCallback? onPressed, {Color? color}) {
    final enabled = onPressed != null;
    return Tooltip(
      message: tooltip,
      child: SizedBox(
        width: 36,
        height: 36,
        child: IconButton(
          icon: Icon(
            icon,
            size: 15,
            color: enabled
                ? (color ?? const Color(0xFF9999bb))
                : const Color(0xFF444455),
          ),
          onPressed: onPressed,
        ),
      ),
    );
  }
}
