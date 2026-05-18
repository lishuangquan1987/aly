import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:publish_tool/models/project_config.dart';
import 'package:publish_tool/viewmodels/app_controller.dart';
import 'package:publish_tool/viewmodels/project_controller.dart';

class ProjectCard extends StatefulWidget {
  final ProjectConfig config;
  final int index;
  const ProjectCard({super.key, required this.config, required this.index});

  @override
  State<ProjectCard> createState() => _ProjectCardState();
}

class _ProjectCardState extends State<ProjectCard> {
  bool _hovered = false;

  @override
  Widget build(BuildContext context) {
    final appCtrl = Get.find<AppController>();
    final isOnline = Get.isRegistered<ProjectController>(tag: widget.config.name)
        ? Get.find<ProjectController>(tag: widget.config.name).serverOsInfo.value != null
        : false;

    return Obx(() {
      final isActive = appCtrl.openTabs.contains(widget.config) &&
          appCtrl.activeTabIndex.value < appCtrl.openTabs.length &&
          appCtrl.openTabs[appCtrl.activeTabIndex.value] == widget.config;

      return MouseRegion(
        onEnter: (_) => setState(() => _hovered = true),
        onExit: (_) => setState(() => _hovered = false),
        cursor: SystemMouseCursors.click,
        child: GestureDetector(
          onTap: () => appCtrl.openTab(widget.config),
          child: Container(
            margin: const EdgeInsets.fromLTRB(8, 3, 8, 3),
            padding: const EdgeInsets.fromLTRB(10, 8, 6, 8),
            decoration: BoxDecoration(
              color: isActive
                  ? const Color(0xFF2a2a45)
                  : _hovered
                      ? const Color(0xFF232334)
                      : const Color(0xFF1e1e2e),
              borderRadius: BorderRadius.circular(6),
              border: Border.all(
                color: isActive ? const Color(0xFF0078d4).withOpacity(0.5) : Colors.transparent,
              ),
            ),
            child: Row(
              children: [
                // 状态点
                Container(
                  width: 6,
                  height: 6,
                  margin: const EdgeInsets.only(right: 8, top: 1),
                  decoration: BoxDecoration(
                    shape: BoxShape.circle,
                    color: isOnline ? const Color(0xFF3fa33f) : const Color(0xFF555566),
                  ),
                ),
                // 信息
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(
                        widget.config.title,
                        style: TextStyle(
                          fontSize: 13,
                          fontWeight: FontWeight.w500,
                          color: isActive ? Colors.white : const Color(0xFFddddee),
                        ),
                        overflow: TextOverflow.ellipsis,
                      ),
                      const SizedBox(height: 2),
                      Text(
                        widget.config.serverUrl,
                        style: const TextStyle(fontSize: 11, color: Color(0xFF5577aa)),
                        overflow: TextOverflow.ellipsis,
                      ),
                    ],
                  ),
                ),
                // 操作按钮（hover时显示）
                if (_hovered || isActive) ...[
                  _iconBtn(FluentIcons.chevron_up, () => appCtrl.moveUp(widget.index)),
                  _iconBtn(FluentIcons.chevron_down, () => appCtrl.moveDown(widget.index)),
                  _iconBtn(FluentIcons.delete, () => appCtrl.deleteProject(widget.index),
                      color: const Color(0xFFaa3333)),
                ],
              ],
            ),
          ),
        ),
      );
    });
  }

  Widget _iconBtn(IconData icon, VoidCallback onPressed, {Color? color}) {
    return IconButton(
      icon: Icon(icon, size: 13, color: color ?? const Color(0xFF777788)),
      onPressed: onPressed,
    );
  }
}
