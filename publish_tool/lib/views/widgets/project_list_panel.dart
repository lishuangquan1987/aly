import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:publish_tool/viewmodels/app_controller.dart';
import 'package:publish_tool/views/project/dialogs/add_project_dialog.dart';
import 'package:publish_tool/views/project/dialogs/add_server_project_dialog.dart';
import 'package:publish_tool/views/widgets/project_card.dart';

class ProjectListPanel extends StatelessWidget {
  const ProjectListPanel({super.key});

  @override
  Widget build(BuildContext context) {
    final ctrl = Get.find<AppController>();
    return Container(
      width: 260,
      color: const Color(0xFF181825),
      child: Column(
        children: [
          // 标题栏
          Container(
            height: 40,
            padding: const EdgeInsets.symmetric(horizontal: 12),
            decoration: const BoxDecoration(
              color: Color(0xFF1e1e2e),
              border: Border(bottom: BorderSide(color: Color(0xFF2a2a3e))),
            ),
            child: Row(
              children: [
                const Icon(FluentIcons.product_list, size: 14, color: Color(0xFF888888)),
                const SizedBox(width: 8),
                const Text('项目列表', style: TextStyle(fontSize: 13, fontWeight: FontWeight.w500)),
                const Spacer(),
                _headerBtn(FluentIcons.server_enviroment, '新建服务器项目', () => _showAddServerDialog(context)),
                const SizedBox(width: 4),
                _headerBtn(FluentIcons.add, '新建客户端项目', () => _showAddDialog(context)),
              ],
            ),
          ),
          // 搜索框
          Padding(
            padding: const EdgeInsets.all(8),
            child: TextBox(
              placeholder: '搜索项目...',
              onChanged: (v) => ctrl.filterKeyword.value = v,
              prefix: const Padding(
                padding: EdgeInsets.only(left: 8),
                child: Icon(FluentIcons.search, size: 13, color: Color(0xFF666666)),
              ),
            ),
          ),
          // 项目列表
          Expanded(
            child: Obx(() {
              final projects = ctrl.filteredProjects;
              if (projects.isEmpty) {
                return const Center(
                  child: Column(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      Icon(FluentIcons.product_list, size: 32, color: Color(0xFF333344)),
                      SizedBox(height: 8),
                      Text('暂无项目', style: TextStyle(color: Color(0xFF555566), fontSize: 12)),
                    ],
                  ),
                );
              }
              return ListView.builder(
                padding: const EdgeInsets.only(bottom: 8),
                itemCount: projects.length,
                itemBuilder: (_, i) {
                  final idx = ctrl.projectConfigs.indexOf(projects[i]);
                  return ProjectCard(config: projects[i], index: idx);
                },
              );
            }),
          ),
        ],
      ),
    );
  }

  Widget _headerBtn(IconData icon, String tooltip, VoidCallback onTap) {
    return Tooltip(
      message: tooltip,
      child: MouseRegion(
        cursor: SystemMouseCursors.click,
        child: GestureDetector(
          onTap: onTap,
          child: Container(
            width: 28,
            height: 28,
            decoration: BoxDecoration(
              color: const Color(0xFF2a2a3e),
              borderRadius: BorderRadius.circular(4),
            ),
            child: Icon(icon, size: 14, color: const Color(0xFF9999bb)),
          ),
        ),
      ),
    );
  }

  void _showAddDialog(BuildContext context) {
    showDialog(context: context, builder: (_) => const AddProjectDialog());
  }

  void _showAddServerDialog(BuildContext context) {
    showDialog(context: context, builder: (_) => const AddServerProjectDialog());
  }
}
