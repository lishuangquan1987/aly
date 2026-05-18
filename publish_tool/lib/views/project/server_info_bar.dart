import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:publish_tool/viewmodels/project_controller.dart';

class ServerInfoBar extends StatelessWidget {
  final String tag;
  const ServerInfoBar({super.key, required this.tag});

  @override
  Widget build(BuildContext context) {
    final ctrl = Get.find<ProjectController>(tag: tag);
    return Obx(() {
      final info = ctrl.serverOsInfo.value;
      if (info == null) {
        return Container(
          height: 52,
          color: const Color(0xFF16213e),
          padding: const EdgeInsets.symmetric(horizontal: 12),
          child: const Align(
            alignment: Alignment.centerLeft,
            child: Text('服务器未连接', style: TextStyle(color: Colors.grey)),
          ),
        );
      }
      final usedPercent = info.diskUsedPercent;
      return Container(
        color: const Color(0xFF16213e),
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                _infoItem('平台', info.platform),
                const SizedBox(width: 16),
                _infoItem('处理器', info.cpuName),
                const SizedBox(width: 16),
                _infoItem('线程', info.numCPU.toString()),
                const SizedBox(width: 16),
                Expanded(
                  child: Row(
                    children: [
                      const Text('磁盘容量 ',
                          style: TextStyle(
                              fontSize: 11, color: Color(0xFF888888))),
                      Expanded(
                        child: ProgressBar(value: usedPercent),
                      ),
                      const SizedBox(width: 8),
                      Text(
                        '${info.diskUsed.toStringAsFixed(1)}G / ${info.diskFree.toStringAsFixed(1)}G / ${info.diskTotal.toStringAsFixed(1)}G',
                        style: const TextStyle(fontSize: 11),
                      ),
                    ],
                  ),
                ),
              ],
            ),
            const SizedBox(height: 4),
            Row(
              children: [
                _infoItem('环境', info.version),
                const SizedBox(width: 16),
                _infoItem('架构', info.goARCH),
                const SizedBox(width: 16),
                _infoItem('频率', '${info.cpuMhz} MHz'),
              ],
            ),
          ],
        ),
      );
    });
  }

  Widget _infoItem(String label, String value) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text('$label ', style: const TextStyle(fontSize: 11, color: Color(0xFF888888))),
        Text(value, style: const TextStyle(fontSize: 11)),
      ],
    );
  }
}
