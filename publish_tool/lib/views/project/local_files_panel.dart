import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:intl/intl.dart';
import 'package:publish_tool/viewmodels/project_controller.dart';

class LocalFilesPanel extends StatelessWidget {
  final String tag;
  const LocalFilesPanel({super.key, required this.tag});

  @override
  Widget build(BuildContext context) {
    final ctrl = Get.find<ProjectController>(tag: tag);
    final dateFmt = DateFormat('MM-dd HH:mm');
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // 面板标题栏
        _PanelHeader(
          title: '本地文件',
          trailing: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Obx(() => Text(
                    ctrl.serverVersion.value.isEmpty
                        ? '暂无版本'
                        : 'v${ctrl.serverVersion.value}',
                    style: const TextStyle(fontSize: 11, color: Color(0xFF5577aa)),
                  )),
              const SizedBox(width: 8),
              _hdrBtn(FluentIcons.folder_open, '打开文件夹', ctrl.openLocalFolder),
              _hdrBtn(FluentIcons.refresh, '扫描文件', ctrl.loadLocalFiles),
            ],
          ),
        ),
        // 过滤框
        Padding(
          padding: const EdgeInsets.fromLTRB(8, 6, 8, 4),
          child: TextBox(
            placeholder: '过滤文件...',
            onChanged: (v) => ctrl.localFileFilter.value = v,
            prefix: const Padding(
              padding: EdgeInsets.only(left: 8),
              child: Icon(FluentIcons.filter, size: 12, color: Color(0xFF666677)),
            ),
          ),
        ),
        // 服务器最新日志（折叠区）
        Obx(() {
          final logs = ctrl.serverChangeLogs;
          if (logs.isEmpty) return const SizedBox.shrink();
          return Container(
            margin: const EdgeInsets.fromLTRB(8, 0, 8, 6),
            padding: const EdgeInsets.all(8),
            decoration: BoxDecoration(
              color: const Color(0xFF0e0e1a),
              borderRadius: BorderRadius.circular(4),
              border: Border.all(color: const Color(0xFF2a2a3e)),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(children: [
                  const Icon(FluentIcons.history, size: 11, color: Color(0xFF666677)),
                  const SizedBox(width: 4),
                  Text('最新变更日志', style: const TextStyle(fontSize: 11, color: Color(0xFF666677))),
                ]),
                const SizedBox(height: 4),
                Text(
                  logs.first.logs.join('\n'),
                  style: const TextStyle(fontSize: 11, color: Color(0xFFbbbbcc)),
                  maxLines: 3,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
            ),
          );
        }),
        // 文件列表
        Expanded(
          child: Obx(() {
            final files = ctrl.filteredLocalFiles;
            if (files.isEmpty) {
              return Center(
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    const Icon(FluentIcons.folder, size: 40, color: Color(0xFF333344)),
                    const SizedBox(height: 12),
                    const Text('暂无文件', style: TextStyle(color: Color(0xFF666677), fontSize: 13)),
                    const SizedBox(height: 12),
                    Button(
                      onPressed: ctrl.loadLocalFiles,
                      child: const Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Icon(FluentIcons.search, size: 13),
                          SizedBox(width: 6),
                          Text('扫描本地文件'),
                        ],
                      ),
                    ),
                  ],
                ),
              );
            }
            return ListView.builder(
              itemCount: files.length,
              itemBuilder: (_, i) {
                final f = files[i];
                return Container(
                  height: 28,
                  padding: const EdgeInsets.symmetric(horizontal: 8),
                  color: i.isEven ? const Color(0xFF191926) : const Color(0xFF161623),
                  child: Row(
                    children: [
                      Checkbox(
                        checked: f.isChecked,
                        onChanged: (v) {
                          f.isChecked = v ?? false;
                          ctrl.localFiles.refresh();
                        },
                      ),
                      const SizedBox(width: 4),
                      Expanded(
                        child: Text(
                          f.relativePath,
                          style: TextStyle(
                            fontSize: 11,
                            color: f.isModified ? const Color(0xFFe8a838) : const Color(0xFFccccdd),
                          ),
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                      Text(
                        dateFmt.format(f.lastModified),
                        style: const TextStyle(fontSize: 10, color: Color(0xFF555566)),
                      ),
                    ],
                  ),
                );
              },
            );
          }),
        ),
      ],
    );
  }

  Widget _hdrBtn(IconData icon, String tooltip, VoidCallback onPressed) {
    return Tooltip(
      message: tooltip,
      child: SizedBox(
        width: 28,
        height: 28,
        child: IconButton(icon: Icon(icon, size: 13), onPressed: onPressed),
      ),
    );
  }
}

class _PanelHeader extends StatelessWidget {
  final String title;
  final Widget? trailing;
  const _PanelHeader({required this.title, this.trailing});

  @override
  Widget build(BuildContext context) {
    return Container(
      height: 36,
      padding: const EdgeInsets.symmetric(horizontal: 10),
      decoration: const BoxDecoration(
        color: Color(0xFF1e1e2e),
        border: Border(bottom: BorderSide(color: Color(0xFF2a2a3e))),
      ),
      child: Row(
        children: [
          Text(title,
              style: const TextStyle(fontSize: 12, fontWeight: FontWeight.w500, color: Color(0xFFaaaacc))),
          const Spacer(),
          if (trailing != null) trailing!,
        ],
      ),
    );
  }
}
