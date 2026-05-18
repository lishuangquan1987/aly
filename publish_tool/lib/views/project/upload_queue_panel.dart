import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:intl/intl.dart';
import 'package:publish_tool/models/upload_file_item.dart';
import 'package:publish_tool/viewmodels/project_controller.dart';

class UploadQueuePanel extends StatelessWidget {
  final String tag;
  const UploadQueuePanel({super.key, required this.tag});

  @override
  Widget build(BuildContext context) {
    final ctrl = Get.find<ProjectController>(tag: tag);
    final dateFmt = DateFormat('MM-dd HH:mm');
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // 面板标题栏
        Container(
          height: 36,
          padding: const EdgeInsets.symmetric(horizontal: 10),
          decoration: const BoxDecoration(
            color: Color(0xFF1e1e2e),
            border: Border(bottom: BorderSide(color: Color(0xFF2a2a3e))),
          ),
          child: const Row(
            children: [
              Text('发布配置',
                  style: TextStyle(fontSize: 12, fontWeight: FontWeight.w500, color: Color(0xFFaaaacc))),
            ],
          ),
        ),
        // 版本号 + 日志输入
        Padding(
          padding: const EdgeInsets.fromLTRB(8, 8, 8, 0),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text('版本号', style: TextStyle(fontSize: 11, color: Color(0xFF888899))),
              const SizedBox(height: 4),
              Row(
                children: [
                  Expanded(
                    child: Obx(() => TextBox(
                          placeholder: '如: 20260417-1020',
                          controller: TextEditingController(text: ctrl.newVersion.value)
                            ..selection = TextSelection.collapsed(offset: ctrl.newVersion.value.length),
                          onChanged: (v) => ctrl.newVersion.value = v,
                        )),
                  ),
                  const SizedBox(width: 4),
                  Tooltip(
                    message: '自动生成版本号',
                    child: SizedBox(
                      width: 32,
                      height: 32,
                      child: IconButton(
                        icon: const Icon(FluentIcons.auto_fill_template, size: 14),
                        onPressed: ctrl.autoGenerateVersion,
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 8),
              const Text('更新日志', style: TextStyle(fontSize: 11, color: Color(0xFF888899))),
              const SizedBox(height: 4),
              SizedBox(
                height: 72,
                child: TextBox(
                  placeholder: '每行一条',
                  maxLines: null,
                  onChanged: (v) => ctrl.newChangeLogs.value = v,
                ),
              ),
            ],
          ),
        ),
        // 待上传文件标题
        Container(
          height: 32,
          margin: const EdgeInsets.only(top: 8),
          padding: const EdgeInsets.symmetric(horizontal: 10),
          decoration: const BoxDecoration(
            border: Border(
              top: BorderSide(color: Color(0xFF2a2a3e)),
              bottom: BorderSide(color: Color(0xFF2a2a3e)),
            ),
          ),
          child: Row(
            children: [
              const Text('待上传文件', style: TextStyle(fontSize: 11, color: Color(0xFF888899))),
              const SizedBox(width: 8),
              Obx(() => Text(
                    '${ctrl.uploadQueue.length}',
                    style: const TextStyle(fontSize: 11, color: Color(0xFF5577aa)),
                  )),
            ],
          ),
        ),
        // 列表
        Expanded(
          child: Obx(() {
            final queue = ctrl.uploadQueue;
            if (queue.isEmpty) {
              return const Center(
                child: Text('队列为空', style: TextStyle(color: Color(0xFF555566), fontSize: 12)),
              );
            }
            return ListView.builder(
              itemCount: queue.length,
              itemBuilder: (_, i) {
                final item = queue[i];
                return Container(
                  height: 28,
                  padding: const EdgeInsets.symmetric(horizontal: 8),
                  color: i.isEven ? const Color(0xFF191926) : const Color(0xFF161623),
                  child: Row(
                    children: [
                      Icon(_statusIcon(item.status), size: 11, color: _statusColor(item.status)),
                      const SizedBox(width: 6),
                      Expanded(
                        child: Text(
                          item.relativePath,
                          style: const TextStyle(fontSize: 11, color: Color(0xFFccccdd)),
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                      Text(
                        dateFmt.format(item.lastModified),
                        style: const TextStyle(fontSize: 10, color: Color(0xFF555566)),
                      ),
                      const SizedBox(width: 4),
                      MouseRegion(
                        cursor: SystemMouseCursors.click,
                        child: GestureDetector(
                          onTap: () => ctrl.removeFromUploadQueue(item),
                          child: const Icon(FluentIcons.chrome_close, size: 9, color: Color(0xFF555566)),
                        ),
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

  IconData _statusIcon(UploadStatus s) => switch (s) {
        UploadStatus.pending => FluentIcons.clock,
        UploadStatus.uploading => FluentIcons.upload,
        UploadStatus.done => FluentIcons.check_mark,
        UploadStatus.failed => FluentIcons.error_badge,
      };

  Color _statusColor(UploadStatus s) => switch (s) {
        UploadStatus.pending => const Color(0xFF666677),
        UploadStatus.uploading => const Color(0xFF0078d4),
        UploadStatus.done => const Color(0xFF3fa33f),
        UploadStatus.failed => const Color(0xFFaa3333),
      };
}
