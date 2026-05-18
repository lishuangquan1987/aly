import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:publish_tool/viewmodels/app_controller.dart';

class ProjectTabBar extends StatelessWidget {
  const ProjectTabBar({super.key});

  @override
  Widget build(BuildContext context) {
    final ctrl = Get.find<AppController>();
    return Obx(() {
      if (ctrl.openTabs.isEmpty) return const SizedBox.shrink();
      return Container(
        height: 34,
        decoration: const BoxDecoration(
          color: Color(0xFF1a1a2a),
          border: Border(bottom: BorderSide(color: Color(0xFF2a2a3e))),
        ),
        child: ListView.builder(
          scrollDirection: Axis.horizontal,
          itemCount: ctrl.openTabs.length,
          itemBuilder: (_, i) {
            final isActive = ctrl.activeTabIndex.value == i;
            return GestureDetector(
              onTap: () => ctrl.activeTabIndex.value = i,
              child: Container(
                constraints: const BoxConstraints(minWidth: 80, maxWidth: 180),
                padding: const EdgeInsets.symmetric(horizontal: 12),
                decoration: BoxDecoration(
                  color: isActive ? const Color(0xFF232336) : Colors.transparent,
                  border: isActive
                      ? const Border(top: BorderSide(color: Color(0xFF0078d4), width: 2))
                      : null,
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Flexible(
                      child: Text(
                        ctrl.openTabs[i].title,
                        style: TextStyle(
                          fontSize: 12,
                          color: isActive ? Colors.white : const Color(0xFF777788),
                        ),
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                    const SizedBox(width: 6),
                    MouseRegion(
                      cursor: SystemMouseCursors.click,
                      child: GestureDetector(
                        onTap: () => ctrl.closeTab(i),
                        child: Icon(
                          FluentIcons.chrome_close,
                          size: 9,
                          color: isActive ? const Color(0xFF888899) : const Color(0xFF555566),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            );
          },
        ),
      );
    });
  }
}
