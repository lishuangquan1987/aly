import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:publish_tool/viewmodels/project_controller.dart';

class BottomActionBar extends StatelessWidget {
  final String tag;
  const BottomActionBar({super.key, required this.tag});

  @override
  Widget build(BuildContext context) {
    final ctrl = Get.find<ProjectController>(tag: tag);
    return Obx(() => Container(
          height: 44,
          decoration: const BoxDecoration(
            color: Color(0xFF1a1a2a),
            border: Border(top: BorderSide(color: Color(0xFF2a2a3e))),
          ),
          padding: const EdgeInsets.symmetric(horizontal: 12),
          child: Row(
            children: [
              // 状态消息
              const Icon(FluentIcons.info, size: 12, color: Color(0xFF555566)),
              const SizedBox(width: 6),
              Expanded(
                child: Text(
                  ctrl.statusMessage.value,
                  style: const TextStyle(fontSize: 11, color: Color(0xFF777788)),
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              // 选项
              Checkbox(
                checked: ctrl.appendToLatest.value,
                onChanged: (v) => ctrl.appendToLatest.value = v ?? false,
                content: Obx(() => Text(
                      '附加到最新版本号: ${ctrl.serverVersion.value}',
                      style: const TextStyle(fontSize: 11),
                    )),
              ),
              const SizedBox(width: 12),
              Checkbox(
                checked: ctrl.autoRefreshAfterPush.value,
                onChanged: (v) => ctrl.autoRefreshAfterPush.value = v ?? true,
                content: const Text('推送成功自动刷新状态', style: TextStyle(fontSize: 11)),
              ),
              const SizedBox(width: 12),
              // 推送按钮
              FilledButton(
                onPressed: ctrl.isBusy.value ? null : ctrl.pushUpdate,
                style: ButtonStyle(
                  padding: WidgetStateProperty.all(
                    const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                  ),
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    if (ctrl.isBusy.value)
                      const SizedBox(
                        width: 12,
                        height: 12,
                        child: ProgressRing(strokeWidth: 2),
                      )
                    else
                      const Icon(FluentIcons.upload, size: 13),
                    const SizedBox(width: 6),
                    const Text('推送更新', style: TextStyle(fontSize: 12)),
                  ],
                ),
              ),
            ],
          ),
        ));
  }
}
